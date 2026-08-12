// internal/schema/validate.go
package schema

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

func (r *Registry) ValidateDocument(ct *ContentType, data map[string]any) error {
	for _, f := range ct.Fields {
		v, present := data[f.Name]
		if !present || v == nil || isEmpty(v) {
			if f.Required {
				return fmt.Errorf("字段 %q 必填", f.Name)
			}
			continue
		}
		if err := r.validateFieldValue(f, v); err != nil {
			return err
		}
	}
	return nil
}

func isEmpty(v any) bool {
	switch t := v.(type) {
	case string:
		return t == ""
	case nil:
		return true
	default:
		return false
	}
}

func (r *Registry) validateFieldValue(f Field, v any) error {
	switch f.Type {
	case TypeText, TypeTextarea, TypeRichText, TypeSlug:
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("字段 %q 需要字符串", f.Name)
		}
		if f.MaxLength > 0 && len([]rune(s)) > f.MaxLength {
			return fmt.Errorf("字段 %q 超长，上限 %d", f.Name, f.MaxLength)
		}
		if f.Pattern != "" {
			re, err := r.getPattern(f.Pattern)
			if err != nil {
				return err
			}
			if !re.MatchString(s) {
				return fmt.Errorf("字段 %q 格式不合法", f.Name)
			}
		}
		if f.Type == TypeSlug && Slugify(s) != s {
			return fmt.Errorf("字段 %q 必须是合法别名", f.Name)
		}
	case TypeNumber:
		n, ok := toFloat(v)
		if !ok {
			return fmt.Errorf("字段 %q 需要数字", f.Name)
		}
		if f.Min != nil && n < *f.Min {
			return fmt.Errorf("字段 %q 不能小于 %v", f.Name, *f.Min)
		}
		if f.Max != nil && n > *f.Max {
			return fmt.Errorf("字段 %q 不能大于 %v", f.Name, *f.Max)
		}
	case TypeBoolean:
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("字段 %q 需要布尔值", f.Name)
		}
	case TypeDate:
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("字段 %q 需要日期字符串", f.Name)
		}
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return fmt.Errorf("字段 %q 日期格式应为 YYYY-MM-DD", f.Name)
		}
	case TypeDateTime:
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("字段 %q 需要时间字符串", f.Name)
		}
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			return fmt.Errorf("字段 %q 时间格式应为 RFC3339", f.Name)
		}
	case TypeSelect:
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("字段 %q 需要字符串", f.Name)
		}
		if !contains(f.Options, s) {
			return fmt.Errorf("字段 %q 值 %q 不在选项内", f.Name, s)
		}
	case TypeMultiSelect:
		items, ok := toStrSlice(v)
		if !ok {
			return fmt.Errorf("字段 %q 需要字符串数组", f.Name)
		}
		for _, item := range items {
			if !contains(f.Options, item) {
				return fmt.Errorf("字段 %q 值 %q 不在选项内", f.Name, item)
			}
		}
	case TypeImage, TypeFile:
		if _, ok := v.(string); !ok {
			return fmt.Errorf("字段 %q 需要字符串(媒体ID或URL)", f.Name)
		}
	case TypeRelation:
		if _, ok := v.(string); !ok {
			return fmt.Errorf("字段 %q 需要字符串(关联内容ID)", f.Name)
		}
	case TypeRepeat:
		arr, ok := v.([]any)
		if !ok {
			return fmt.Errorf("字段 %q 需要数组", f.Name)
		}
		for i, item := range arr {
			row, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("字段 %q 第 %d 行需为对象", f.Name, i+1)
			}
			for _, sub := range f.SubFields {
				// M7：缺值/空值先判 Required/isEmpty 再走类型断言，空字符串必填子字段报必填而非类型错误。
				sv, present := row[sub.Name]
				if (!present || isEmpty(sv)) && sub.Required {
					return fmt.Errorf("字段 %q 第 %d 行: 字段 %q 必填", f.Name, i+1, sub.Name)
				}
				if present && !isEmpty(sv) {
					if err := r.validateFieldValue(sub, sv); err != nil {
						return fmt.Errorf("字段 %q 第 %d 行: %w", f.Name, i+1, err)
					}
				}
			}
		}
	default:
		return fmt.Errorf("字段 %q 类型 %q 不支持", f.Name, f.Type)
	}
	return nil
}

func (r *Registry) getPattern(p string) (*regexp.Regexp, error) {
	r.patternsMu.Lock()
	re, ok := r.patterns[p]
	r.patternsMu.Unlock()
	if ok {
		return re, nil
	}
	re, err := regexp.Compile(p)
	if err != nil {
		return nil, err
	}
	r.patternsMu.Lock()
	r.patterns[p] = re
	r.patternsMu.Unlock()
	return re, nil
}

func toStrSlice(v any) ([]string, bool) {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, fmt.Sprint(item))
		}
		return out, true
	case []string:
		return t, true
	default:
		return nil, false
	}
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		n, err := t.Float64()
		return n, err == nil
	case string:
		n, err := strconv.ParseFloat(t, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// validateSubFields 递归校验 repeat 子字段组（名称/类型/嵌套）。
func (r *Registry) validateSubFields(fields []Field) error {
	seen := map[string]bool{}
	for i := range fields {
		f := &fields[i]
		if !validName.MatchString(f.Name) {
			return fmt.Errorf("字段名 %q 不合法", f.Name)
		}
		if seen[f.Name] {
			return fmt.Errorf("字段名 %q 重复", f.Name)
		}
		seen[f.Name] = true
		// I4：子字段类型须为注册的合法类型（与顶层 ValidateContentType 一致）。
		if !r.IsValid(f.Type) {
			return fmt.Errorf("字段 %q 类型 %q 不支持", f.Name, f.Type)
		}
		switch f.Type {
		case TypeRelation:
			if f.RelationType == "" {
				return fmt.Errorf("字段 %q 必须配置目标内容类型", f.Name)
			}
		case TypeRepeat:
			// I4：嵌套 repeat 空子字段拒绝（与顶层 len==0 检查一致）。
			if len(f.SubFields) == 0 {
				return fmt.Errorf("字段 %q 必须配置子字段", f.Name)
			}
			if err := r.validateSubFields(f.SubFields); err != nil {
				return fmt.Errorf("字段 %q 子字段配置错误: %w", f.Name, err)
			}
		}
	}
	return nil
}
