// internal/schema/schema.go
package schema

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

type FieldType string

const (
	TypeText        FieldType = "text"
	TypeTextarea    FieldType = "textarea"
	TypeRichText    FieldType = "richtext"
	TypeNumber      FieldType = "number"
	TypeBoolean     FieldType = "boolean"
	TypeDate        FieldType = "date"
	TypeDateTime    FieldType = "datetime"
	TypeSelect      FieldType = "select"
	TypeMultiSelect FieldType = "multiselect"
	TypeImage       FieldType = "image"
	TypeFile        FieldType = "file"
	TypeSlug        FieldType = "slug"
	TypeRelation    FieldType = "relation"
	TypeRepeat      FieldType = "repeat"
)

type Field struct {
	Name         string    `json:"name"`
	Label        string    `json:"label"`
	Type         FieldType `json:"type"`
	Required     bool      `json:"required,omitempty"`
	Indexed      bool      `json:"indexed,omitempty"`
	Translatable bool      `json:"translatable,omitempty"`
	Default      any       `json:"default,omitempty"`
	Options      []string  `json:"options,omitempty"`
	MaxLength    int       `json:"max_length,omitempty"`
	Pattern      string    `json:"pattern,omitempty"`
	Min          *float64  `json:"min,omitempty"`
	Max          *float64  `json:"max,omitempty"`
	RelationType string    `json:"relation_type,omitempty"` // relation: 目标内容类型名
	SubFields    []Field   `json:"sub_fields,omitempty"`    // repeat: 子字段组
}

type ContentType struct {
	ID     int64          `json:"id"`
	Name   string         `json:"name"`
	Label  string         `json:"label"`
	Fields []Field        `json:"fields"`
	Config map[string]any `json:"config,omitempty"`
}

type Registry struct {
	types      map[FieldType]bool
	order      []FieldType
	patterns   map[string]*regexp.Regexp
	patternsMu sync.Mutex
}

func NewRegistry() *Registry {
	types := []FieldType{
		TypeText, TypeTextarea, TypeRichText, TypeNumber, TypeBoolean,
		TypeDate, TypeDateTime, TypeSelect, TypeMultiSelect, TypeImage,
		TypeFile, TypeSlug, TypeRelation, TypeRepeat,
	}
	r := &Registry{types: map[FieldType]bool{}, patterns: map[string]*regexp.Regexp{}}
	for _, t := range types {
		r.types[t] = true
		r.order = append(r.order, t)
	}
	return r
}

func (r *Registry) IsValid(t FieldType) bool { return r.types[t] }

func (r *Registry) AllTypes() []FieldType { return r.order }

var validName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

func (r *Registry) ValidateContentType(ct *ContentType) error {
	if !validName.MatchString(ct.Name) {
		return fmt.Errorf("内容类型名 %q 不合法，需以字母开头且只含字母数字下划线", ct.Name)
	}
	seen := map[string]bool{}
	for i := range ct.Fields {
		f := &ct.Fields[i]
		if !validName.MatchString(f.Name) {
			return fmt.Errorf("字段名 %q 不合法", f.Name)
		}
		if seen[f.Name] {
			return fmt.Errorf("字段名 %q 重复", f.Name)
		}
		seen[f.Name] = true
		if !r.IsValid(f.Type) {
			return fmt.Errorf("字段 %q 类型 %q 不支持", f.Name, f.Type)
		}
		if (f.Type == TypeSelect || f.Type == TypeMultiSelect) && len(f.Options) == 0 {
			return fmt.Errorf("select 字段 %q 必须配置选项", f.Name)
		}
		if f.Type == TypeRelation && f.RelationType == "" {
			return fmt.Errorf("字段 %q 必须配置目标内容类型", f.Name)
		}
		if f.Type == TypeRepeat {
			if len(f.SubFields) == 0 {
				return fmt.Errorf("字段 %q 必须配置子字段", f.Name)
			}
			if err := r.validateSubFields(f.SubFields); err != nil {
				return fmt.Errorf("字段 %q 子字段配置错误: %w", f.Name, err)
			}
		}
		if f.Indexed && (f.Type == TypeRelation || f.Type == TypeRepeat) {
			return fmt.Errorf("字段 %q 类型 %q 不支持作为索引列", f.Name, f.Type)
		}
	}
	return nil
}

// IndexField 返回被标记为索引列(冗余到 content.title)的字段，没有则返回 nil。
func IndexField(ct *ContentType) *Field {
	for i := range ct.Fields {
		f := &ct.Fields[i]
		if f.Indexed && (f.Type == TypeText || f.Type == TypeTextarea) {
			return f
		}
	}
	return nil
}

// DecodeFields 把 content_types.fields 的 JSON 解码为 Field 列表。
func DecodeFields(raw string) ([]Field, error) {
	var fs []Field
	if err := json.Unmarshal([]byte(raw), &fs); err != nil {
		return nil, err
	}
	return fs, nil
}

// Slugify 生成简单 ASCII slug：转小写、非字母数字转横线、合并横线、去首尾。
func Slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
