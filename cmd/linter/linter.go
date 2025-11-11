// main пакет анализатора.
package main

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/singlechecker"
)

// Экземпляр анализатора.
var analyzer = &analysis.Analyzer{
	Name: "linter",
	Doc:  "reports use of panic, log.Fatal and os.Exit",
	Run:  run,
}

// run, реализует функционал анализа. Возвращает интерфейс и ошибку.
//
// Параметры:
//
//	pass - контекст анализа.
func run(pass *analysis.Pass) (interface{}, error) {

	for _, file := range pass.Files {

		// Пропуск файлов пакета main
		if file.Name.Name == "main" {
			continue
		}

		// Исследование
		ast.Inspect(file, func(n ast.Node) bool {
			callExpr, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Проверка на panic
			ident, ok := callExpr.Fun.(*ast.Ident)
			if ok && ident.Name == "panic" {
				pass.Reportf(callExpr.Pos(), "найдено использование panic")
				return true
			}

			// Проверка на log.Fatal и os.Exit
			selector, ok := callExpr.Fun.(*ast.SelectorExpr)
			if ok && selector.X != nil {
				xIdent, ok := selector.X.(*ast.Ident)
				if ok {
					if xIdent.Name == "log" && selector.Sel.Name == "Fatal" {
						pass.Reportf(callExpr.Pos(), "найдено использование log.Fatal")
						return true
					}

					if xIdent.Name == "os" && selector.Sel.Name == "Exit" {
						pass.Reportf(callExpr.Pos(), "найдено использование os.Exit")
						return true
					}
				}
			}
			return true
		})
	}
	return nil, nil
}

// main, основная функция.
func main() {
	singlechecker.Main(analyzer)
}
