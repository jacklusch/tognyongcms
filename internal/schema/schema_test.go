package schema

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func validCT() ContentType {
	return ContentType{
		Name:  "article",
		Label: "文章",
		Fields: []Field{
			{Name: "title", Label: "标题", Type: TypeText, Required: true, Indexed: true, MaxLength: 100},
			{Name: "slug", Label: "别名", Type: TypeSlug, Required: true},
			{Name: "content", Label: "正文", Type: TypeRichText, Translatable: true},
			{Name: "views", Label: "浏览数", Type: TypeNumber, Min: floatPtr(0)},
			{Name: "visible", Label: "可见", Type: TypeBoolean},
			{Name: "tag", Label: "标签", Type: TypeSelect, Options: []string{"go", "web"}},
			{Name: "published_on", Label: "发布日期", Type: TypeDate},
		},
	}
}

func floatPtr(f float64) *float64 { return &f }

func TestValidateContentType(t *testing.T) {
	reg := NewRegistry()

	good := validCT()
	if err := reg.ValidateContentType(&good); err != nil {
		t.Fatalf("valid CT should pass: %v", err)
	}

	dup := validCT()
	dup.Fields[0].Name = "content"
	if err := reg.ValidateContentType(&dup); err == nil {
		t.Error("重复字段名应报错")
	}

	bad := validCT()
	bad.Fields[0].Type = FieldType("nope")
	if err := reg.ValidateContentType(&bad); err == nil {
		t.Error("非法字段类型应报错")
	}
}

func TestValidateDocument(t *testing.T) {
	reg := NewRegistry()
	ct := validCT()

	ok := map[string]any{
		"title": "你好", "slug": "hello", "content": "<p>hi</p>",
		"views": 3.0, "visible": true, "tag": "go", "published_on": "2026-08-05",
	}
	if err := reg.ValidateDocument(&ct, ok); err != nil {
		t.Fatalf("valid doc should pass: %v", err)
	}

	missing := map[string]any{}
	if err := reg.ValidateDocument(&ct, missing); err == nil {
		t.Error("缺必填字段应报错")
	}

	tooLong := map[string]any{"title": "超长标题", "slug": "x"}
	// 构造超长 title
	tooLong["title"] = string(make([]byte, 200))
	if err := reg.ValidateDocument(&ct, tooLong); err == nil {
		t.Error("超长 title 应报错")
	}

	badSelect := map[string]any{"title": "t", "slug": "s", "tag": "python"}
	if err := reg.ValidateDocument(&ct, badSelect); err == nil {
		t.Error("非法 select 值应报错")
	}

	badNum := map[string]any{"title": "t", "slug": "s", "views": -1.0}
	if err := reg.ValidateDocument(&ct, badNum); err == nil {
		t.Error("小于 min 的数字应报错")
	}

	badDate := map[string]any{"title": "t", "slug": "s", "published_on": "not-a-date"}
	if err := reg.ValidateDocument(&ct, badDate); err == nil {
		t.Error("非法日期应报错")
	}
}

func TestValidateDocumentCoverage(t *testing.T) {
	reg := NewRegistry()
	ct := ContentType{Name: "post", Label: "帖子", Fields: []Field{
		{Name: "title", Label: "标题", Type: TypeText, Required: true},
		{Name: "pub", Label: "时间", Type: TypeDateTime},
		{Name: "email", Label: "邮箱", Type: TypeText, Pattern: `^[^@]+@[^@]+$`},
		{Name: "tags", Label: "标签", Type: TypeMultiSelect, Options: []string{"go", "web"}},
	}}
	cases := []struct {
		name string
		data map[string]any
		want bool
	}{
		{"合法", map[string]any{"title": "t", "pub": "2026-08-06T10:00:00Z", "email": "a@b.com", "tags": []string{"go"}}, true},
		{"缺必填", map[string]any{}, false},
		{"datetime 非法", map[string]any{"title": "t", "pub": "2026/08/06"}, false},
		{"pattern 不匹配", map[string]any{"title": "t", "email": "nope"}, false},
		{"multiselect 值不在选项", map[string]any{"title": "t", "tags": []any{"rust"}}, false},
	}
	for _, c := range cases {
		err := reg.ValidateDocument(&ct, c.data)
		if (err == nil) != c.want {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.want)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Hello World": "hello-world",
		"你好":          "",
		"Go & Web":    "go-web",
		"  spaced  ":  "spaced",
		"a--b":        "a-b",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIndexFieldAndDecode(t *testing.T) {
	ct := ContentType{Fields: []Field{
		{Name: "title", Type: TypeText, Indexed: true},
		{Name: "body", Type: TypeRichText},
	}}
	f := IndexField(&ct)
	if f == nil || f.Name != "title" {
		t.Errorf("IndexField = %+v", f)
	}
	if got := IndexField(&ContentType{}); got != nil {
		t.Errorf("无索引字段应为 nil")
	}
	fs, err := DecodeFields(`[{"name":"title","type":"text"}]`)
	if err != nil || len(fs) != 1 || fs[0].Name != "title" {
		t.Errorf("DecodeFields = %+v, %v", fs, err)
	}
}

// TestGetPatternConcurrent 并发校验带 pattern 字段的文档，验证 pattern 缓存无并发写竞态。
func TestGetPatternConcurrent(t *testing.T) {
	reg := NewRegistry()
	const goroutines, iters = 20, 50
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		// 每个 goroutine 用独立 pattern，保证首轮全部走缓存未命中→并发写路径
		ct := ContentType{
			Name:  "post",
			Label: "帖子",
			Fields: []Field{
				{Name: "code", Label: "编码", Type: TypeText, Required: true, Pattern: fmt.Sprintf(`^g%d$`, i)},
			},
		}
		go func(i int) {
			defer wg.Done()
			<-start
			for j := 0; j < iters; j++ {
				if err := reg.ValidateDocument(&ct, map[string]any{"code": fmt.Sprintf("g%d", i)}); err != nil {
					t.Errorf("goroutine %d iter %d: %v", i, j, err)
					return
				}
			}
		}(i)
	}
	close(start)
	wg.Wait()
}

func TestAllTypes(t *testing.T) {
	reg := NewRegistry()
	all := reg.AllTypes()
	if len(all) != 14 {
		t.Errorf("AllTypes = %d, want 14", len(all))
	}
	for _, ft := range all {
		if !reg.IsValid(ft) {
			t.Errorf("IsValid(%s) = false", ft)
		}
	}
	if reg.IsValid(FieldType("nope")) {
		t.Error("非法类型应无效")
	}
}

func TestValidateContentTypeRelationRepeat(t *testing.T) {
	reg := NewRegistry()
	good := ContentType{Name: "post", Label: "帖子", Fields: []Field{
		{Name: "author", Label: "作者", Type: TypeRelation, RelationType: "user"},
		{Name: "items", Label: "条目", Type: TypeRepeat, SubFields: []Field{
			{Name: "title", Label: "标题", Type: TypeText, Required: true},
		}},
	}}
	if err := reg.ValidateContentType(&good); err != nil {
		t.Fatalf("合法 CT 应通过: %v", err)
	}
	// relation 缺目标类型
	badRel := ContentType{Name: "x1", Fields: []Field{{Name: "author", Type: TypeRelation}}}
	if err := reg.ValidateContentType(&badRel); err == nil {
		t.Error("relation 缺目标类型应报错")
	}
	// repeat 缺子字段
	badRep := ContentType{Name: "x2", Fields: []Field{{Name: "items", Type: TypeRepeat}}}
	if err := reg.ValidateContentType(&badRep); err == nil {
		t.Error("repeat 缺子字段应报错")
	}
	// repeat 子字段非法（重复名）
	badSub := ContentType{Name: "x3", Fields: []Field{{Name: "items", Type: TypeRepeat, SubFields: []Field{
		{Name: "a", Type: TypeText}, {Name: "a", Type: TypeText},
	}}}}
	if err := reg.ValidateContentType(&badSub); err == nil {
		t.Error("repeat 子字段重名应报错")
	}
	// relation/repeat 不能做索引
	idx := ContentType{Name: "x4", Fields: []Field{{Name: "r", Type: TypeRelation, Indexed: true}}}
	if err := reg.ValidateContentType(&idx); err == nil {
		t.Error("relation 不应可索引")
	}
}

func TestValidateContentTypeSubFieldTypeAndNestedRepeat(t *testing.T) {
	reg := NewRegistry()
	// I4：repeat 子字段类型非法 → 报错
	badType := ContentType{Name: "x5", Fields: []Field{{Name: "items", Type: TypeRepeat, SubFields: []Field{
		{Name: "a", Type: FieldType("nope")},
	}}}}
	if err := reg.ValidateContentType(&badType); err == nil {
		t.Error("repeat 子字段类型非法应报错")
	}
	// I4：嵌套 repeat 子字段为空 → 报错（与顶层 len==0 一致）
	nestedEmpty := ContentType{Name: "x6", Fields: []Field{{Name: "items", Type: TypeRepeat, SubFields: []Field{
		{Name: "group", Type: TypeRepeat},
	}}}}
	if err := reg.ValidateContentType(&nestedEmpty); err == nil {
		t.Error("嵌套 repeat 缺子字段应报错")
	}
	// 合法嵌套仍通过
	ok := ContentType{Name: "x7", Fields: []Field{{Name: "items", Type: TypeRepeat, SubFields: []Field{
		{Name: "group", Type: TypeRepeat, SubFields: []Field{{Name: "name", Type: TypeText}}},
	}}}}
	if err := reg.ValidateContentType(&ok); err != nil {
		t.Errorf("合法嵌套应通过: %v", err)
	}
}

func TestValidateDocumentRepeatEmptyRequiredSubField(t *testing.T) {
	reg := NewRegistry()
	ct := ContentType{Name: "post", Fields: []Field{
		{Name: "items", Label: "条目", Type: TypeRepeat, SubFields: []Field{
			{Name: "title", Label: "标题", Type: TypeText, Required: true},
		}},
	}}
	// M7：行内子字段为空字符串且必填 → 报必填（非类型错误）
	err := reg.ValidateDocument(&ct, map[string]any{"items": []any{map[string]any{"title": ""}}})
	if err == nil {
		t.Fatal("行内空字符串必填子字段应报错")
	}
	if !strings.Contains(err.Error(), "必填") {
		t.Errorf("应报必填而非类型错误: %v", err)
	}
	// 行内子字段缺失且必填 → 报必填
	if err := reg.ValidateDocument(&ct, map[string]any{"items": []any{map[string]any{}}}); err == nil {
		t.Error("行内缺必填子字段应报错")
	}
	// 非必填子字段为空 → 不报错
	opt := ContentType{Name: "post2", Fields: []Field{
		{Name: "items", Label: "条目", Type: TypeRepeat, SubFields: []Field{
			{Name: "title", Label: "标题", Type: TypeText},
		}},
	}}
	if err := reg.ValidateDocument(&opt, map[string]any{"items": []any{map[string]any{"title": ""}}}); err != nil {
		t.Errorf("非必填空子字段不应报错: %v", err)
	}
}

func TestValidateDocumentRelationRepeat(t *testing.T) {
	reg := NewRegistry()
	ct := ContentType{Name: "post", Fields: []Field{
		{Name: "author", Label: "作者", Type: TypeRelation, RelationType: "user", Required: true},
		{Name: "items", Label: "条目", Type: TypeRepeat, SubFields: []Field{
			{Name: "title", Label: "标题", Type: TypeText, Required: true},
		}},
	}}
	// 合法
	if err := reg.ValidateDocument(&ct, map[string]any{
		"author": "grp-abc",
		"items":  []any{map[string]any{"title": "一"}},
	}); err != nil {
		t.Fatalf("合法文档应通过: %v", err)
	}
	// relation 空
	if err := reg.ValidateDocument(&ct, map[string]any{"author": "", "items": []any{}}); err == nil {
		t.Error("必填 relation 为空应报错")
	}
	// repeat 行内缺必填子字段
	if err := reg.ValidateDocument(&ct, map[string]any{"author": "g", "items": []any{map[string]any{}}}); err == nil {
		t.Error("repeat 行缺必填子字段应报错")
	}
	// repeat 非数组
	if err := reg.ValidateDocument(&ct, map[string]any{"author": "g", "items": "nope"}); err == nil {
		t.Error("repeat 非数组应报错")
	}
}
