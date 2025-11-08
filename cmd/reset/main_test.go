package main

import (
	"go/ast"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_FormatType_SUCCESS(t *testing.T) {
	tests := []struct {
		expr     ast.Expr
		expected string
	}{
		{&ast.Ident{Name: "int"}, "int"},
		{&ast.StarExpr{X: &ast.Ident{Name: "MyStruct"}}, "*MyStruct"},
		{&ast.ArrayType{Elt: &ast.Ident{Name: "int"}}, "[]int"},
		{&ast.MapType{Key: &ast.Ident{Name: "string"}, Value: &ast.Ident{Name: "int"}}, "map[string]int"},
		{&ast.Ident{Name: "float64"}, "float64"},
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			result := formatType(test.expr)
			assert.Equalf(t, test.expected, result, "ожидалось <%s>, а получено <%s>", test.expected, result)
		})
	}
}

func Test_ResetField_SUCCESS(t *testing.T) {
	tests := []struct {
		fieldName string
		fieldType ast.Expr
		expected  string
	}{
		{"Field1", &ast.Ident{Name: "int"}, "    rs.Field1 = 0\n"},
		{"Field2", &ast.Ident{Name: "string"}, "    rs.Field2 = \"\"\n"},
		{"Field3", &ast.Ident{Name: "MyStruct"}, "    rs.Field3 = MyStruct{}\n"},
		{"Field4", &ast.StarExpr{X: &ast.Ident{Name: "int"}}, "    if rs.Field4 != nil {\n        *rs.Field4 = 0\n    }\n"},
		{"Field5", &ast.ArrayType{Elt: &ast.Ident{Name: "string"}}, "    rs.Field5 = rs.Field5[:0]\n"},
		{"Field6", &ast.MapType{Key: &ast.Ident{Name: "string"}, Value: &ast.Ident{Name: "int"}}, "    clear(rs.Field6)\n"},
		{"Field7", &ast.StructType{}, "    if resetter, ok := rs.Field7.(interface{ Reset() }); ok && rs.Field7 != nil {\n        resetter.Reset()\n    }\n"},
	}

	for _, test := range tests {
		t.Run(test.fieldName, func(t *testing.T) {
			var sb strings.Builder
			resetField(&sb, test.fieldName, test.fieldType)
			assert.Equal(t, test.expected, sb.String(), "Output should match expected")
		})
	}
}

func Test_GenerateResetMethod_SUCCESS(t *testing.T) {
	tests := []struct {
		structName string
		fields     *ast.FieldList
		expected   string
	}{
		{
			structName: "MyStruct",
			fields: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("Field1")},
						Type:  &ast.Ident{Name: "int"},
					},
					{
						Names: []*ast.Ident{ast.NewIdent("Field2")},
						Type:  &ast.Ident{Name: "string"},
					},
				},
			},
			expected: `func (rs *MyStruct) Reset() {
    if rs == nil {
        return
    }
    rs.Field1 = 0
    rs.Field2 = ""
}

`,
		},
		{
			structName: "AnotherStruct",
			fields: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("Field3")},
						Type:  &ast.StarExpr{X: &ast.Ident{Name: "int"}},
					},
					{
						Names: []*ast.Ident{ast.NewIdent("Field4")},
						Type:  &ast.ArrayType{Elt: &ast.Ident{Name: "string"}},
					},
				},
			},
			expected: `func (rs *AnotherStruct) Reset() {
    if rs == nil {
        return
    }
    if rs.Field3 != nil {
        *rs.Field3 = 0
    }
    rs.Field4 = rs.Field4[:0]
}

`,
		},
	}

	for _, test := range tests {
		t.Run(test.structName, func(t *testing.T) {
			var sb strings.Builder
			generateResetMethod(&sb, test.structName, test.fields)
			assert.Equal(t, test.expected, sb.String(), "Output should match expected")
		})
	}
}

func Test_HasGenerateResetComment_SUCCESS(t *testing.T) {
	tests := []struct {
		comment  *ast.CommentGroup
		expected bool
	}{
		{ // Тест 1: комментарий с "generate:reset"
			comment: &ast.CommentGroup{
				List: []*ast.Comment{
					{Text: "// generate:reset"},
				},
			},
			expected: true,
		},
		{ // Тест 2: комментарий без "generate:reset"
			comment: &ast.CommentGroup{
				List: []*ast.Comment{
					{Text: "// some other comment"},
				},
			},
			expected: false,
		},
		{ // Тест 3: пустая группа комментариев
			comment:  nil,
			expected: false,
		},
		{ // Тест 4: несколько комментариев, один содержит "generate:reset"
			comment: &ast.CommentGroup{
				List: []*ast.Comment{
					{Text: "// this is a comment"},
					{Text: "// generate:reset"},
				},
			},
			expected: true,
		},
		{ // Тест 5: несколько комментариев, ни один не содержит "generate:reset"
			comment: &ast.CommentGroup{
				List: []*ast.Comment{
					{Text: "// comment 1"},
					{Text: "// comment 2"},
				},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run("Testing comments", func(t *testing.T) {
			genDecl := &ast.GenDecl{Doc: test.comment}
			result := hasGenerateResetComment(genDecl)
			assert.Equal(t, test.expected, result, "Result should match expected")
		})
	}
}
