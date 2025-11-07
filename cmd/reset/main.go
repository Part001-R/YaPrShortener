// main, содержит реализацию по генерации кода, на сброс содержимого экземпляров структур.
package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// main, главная функция пакета.
func main() {
	const projectRoot = "."

	fset := token.NewFileSet()

	err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Пропуск директории и файлов не .go.
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Пропуск тестовых файлов, файлов с примерами и уже сгенерированных.
		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "gen.go") {
			return nil
		}

		// Парсинг файла
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			fmt.Printf("Ошибка парсинга файла %s: %v\n", path, err)
			return nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("package %s\n\n", node.Name.Name))

		hasStructs := false

		// Обработка файла
		for _, decl := range node.Decls {
			genDecl, ok := decl.(*ast.GenDecl)

			// Пропуск не-type декларации (var, const, func и т.п.)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}

			// Проверка присутствия комментария // generate:reset
			if !hasGenerateResetComment(genDecl) {
				continue
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				structType, ok := typeSpec.Type.(*ast.StructType)
				if ok {
					generateResetMethod(&sb, typeSpec.Name.Name, structType.Fields)
					hasStructs = true // установка признака, что подходящая структура найдена
				}
			}
		}

		// Запись в файл, если флаг обнаружения структуры взведён.
		if hasStructs {
			formattedCode, err := format.Source([]byte(sb.String()))
			if err != nil {
				fmt.Printf("Ошибка форматирования кода для %s: %v\n", path, err)
				return nil
			}

			dir := filepath.Dir(path)
			outputPath := filepath.Join(dir, "reset.gen.go")

			if err = os.WriteFile(outputPath, formattedCode, 0644); err != nil {
				fmt.Printf("Ошибка записи файла %s: %v\n", outputPath, err)
				return nil
			}

			fmt.Printf("Код сброса сгенерирован: %s\n", outputPath)
		}

		return nil
	})

	if err != nil {
		fmt.Println("Ошибка при обходе файлов:", err)
		return
	}

	fmt.Println("Генерация завершена.")
}

// hasGenerateResetComment, проверяет присутствие комментария generate:reset. Возвращает true - если есть обнаружение, false - нет.
//
// Параметры:
//
//	node - анализируемый узел AST.
func hasGenerateResetComment(node *ast.GenDecl) bool {
	if node.Doc == nil {
		return false
	}
	for _, comment := range node.Doc.List {
		if strings.Contains(comment.Text, "// generate:reset") {
			return true
		}
	}
	return false
}

// generateResetMethod, генерирует метод для сброса значений у полей экземпляра структуры.
//
// Параметры:
//
//	sb - указатель на strings.Builder.
//	structName - имя структуры, для которой будет сгенерирован метод.
//	fields - список полей структуры.
func generateResetMethod(sb *strings.Builder, structName string, fields *ast.FieldList) {
	sb.WriteString(fmt.Sprintf("func (rs *%s) Reset() {\n", structName))
	sb.WriteString("    if rs == nil {\n")
	sb.WriteString("        return\n")
	sb.WriteString("    }\n")

	for _, field := range fields.List {
		for _, name := range field.Names {
			resetField(sb, name.Name, field.Type)
		}
	}

	sb.WriteString("}\n\n")
}

//	resetField, генерирует код сброса значения поля, согласно его типа.
//
// Параметры:
//
//	sb - указатель на strings.Builder.
//	fieldName - имя поля.
//	fieldType - тип поля.
func resetField(sb *strings.Builder, fieldName string, fieldType ast.Expr) {
	switch t := fieldType.(type) {
	case *ast.Ident:
		switch t.Name {
		case "int", "int64", "int32", "int16", "int8",
			"uint", "uint64", "uint32", "uint16", "uint8",
			"float32", "float64":
			sb.WriteString(fmt.Sprintf("    rs.%s = 0\n", fieldName))
		case "string":
			sb.WriteString(fmt.Sprintf("    rs.%s = \"\"\n", fieldName))
		default:
			sb.WriteString(fmt.Sprintf("    rs.%s = %s{}\n", fieldName, t.Name))
		}
	case *ast.StarExpr:
		innerType, ok := t.X.(*ast.Ident)
		if ok && (innerType.Name == "string" || innerType.Name == "int" ||
			innerType.Name == "int64" || innerType.Name == "int32" ||
			innerType.Name == "int16" || innerType.Name == "int8" ||
			innerType.Name == "uint" || innerType.Name == "uint64" ||
			innerType.Name == "uint32" || innerType.Name == "uint16" ||
			innerType.Name == "uint8" || innerType.Name == "float32" ||
			innerType.Name == "float64") {
			sb.WriteString(fmt.Sprintf("    if rs.%s != nil {\n", fieldName))
			sb.WriteString(fmt.Sprintf("        *rs.%s = \"\"\n", fieldName))
			sb.WriteString("    }\n")
		} else {
			sb.WriteString(fmt.Sprintf("    if resetter, ok := rs.%s.(interface{ Reset() }); ok && rs.%s != nil {\n", fieldName, fieldName))
			sb.WriteString("        resetter.Reset()\n")
			sb.WriteString("    }\n")
		}
	case *ast.ArrayType:
		sb.WriteString(fmt.Sprintf("    rs.%s = rs.%s[:0]\n", fieldName, fieldName))
	case *ast.MapType:
		sb.WriteString(fmt.Sprintf("    clear(rs.%s)\n", fieldName))
	case *ast.StructType:
		sb.WriteString(fmt.Sprintf("    if resetter, ok := rs.%s.(interface{ Reset() }); ok && rs.%s != nil {\n", fieldName, fieldName))
		sb.WriteString("        resetter.Reset()\n")
		sb.WriteString("    }\n")
	default:
		typeStr := formatType(fieldType)
		sb.WriteString(fmt.Sprintf("    rs.%s = %s{}\n", fieldName, typeStr))
	}
}

// formatType, преобразует абстрактное синтаксическое дерево в удобный для чтения формат. Возвращает строковое представление типа.
//
// Параметры:
//
//	expr - Интерфейс, представляющий тип выражения.
func formatType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + formatType(t.X)
	case *ast.ArrayType:
		return "[]" + formatType(t.Elt)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", formatType(t.Key), formatType(t.Value))
	default:
		return "interface{}"
	}
}
