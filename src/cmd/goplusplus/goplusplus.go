package main

import (
	"flag"
	"fmt"
	"github.com/Kouzukii/goplusplus/src/go/ast"
	"github.com/Kouzukii/goplusplus/src/go/parser"
	"github.com/Kouzukii/goplusplus/src/go/printer"
	"github.com/Kouzukii/goplusplus/src/go/token"
	"io"
	"io/ioutil"
	"os"
)

var (
	output = flag.String("o", "", "output to go file")
)

var (
	fileSet = token.NewFileSet()
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: go++ [flags] [path ...]\n")
	flag.PrintDefaults()
}

type transpiler struct {
	file *ast.File
}

func (t *transpiler) addImport(imp string) {
	for _, i := range t.file.Imports {
		if i.Name.Name == imp {
			return
		}
	}
	t.file.Imports = append(t.file.Imports, &ast.ImportSpec{Name: ast.NewIdent(imp)})
}

func (t *transpiler) transpileDecl(decl *ast.Decl) {
	switch u := (*decl).(type) {
	case *ast.FuncDecl:
		for _, stmt := range u.Body.List {
			t.transpileStmt(&stmt)
		}
	case *ast.GenDecl:
		for _, spec := range u.Specs {
			if k, ok := spec.(*ast.ValueSpec); ok {
				for _, v := range k.Values {
					t.transpileExpr(&v)
				}
			}
		}
	}
}

func (t *transpiler) transpileExprs(exprs []ast.Expr) {
	for _, v := range exprs {
		t.transpileExpr(&v)
	}
}

func (t *transpiler) transpileExpr(expr *ast.Expr) {
	switch u := (*expr).(type) {
	case *ast.CallExpr:
		t.transpileExprs(u.Args)
	case *ast.InterpLit:
		fmtstr := ""
		args := make([]ast.Expr, 1, len(u.Segments)+1)
		for _, s := range u.Segments {
			if s.X != nil {
				if s.Fmt == "" {
					fmtstr = fmt.Sprintf("%s%%v", fmtstr)
				} else {
					fmtstr = fmt.Sprint("%s%%%s", fmtstr, s.Fmt)
				}
				args = append(args, s.X)
			} else {
				fmtstr = fmt.Sprintf("%s%s", fmtstr, s.Fmt)
			}
		}
		args[0] = &ast.BasicLit{Kind: token.STRING, Value: fmtstr}
		*expr = &ast.CallExpr{Fun: &ast.BasicLit{Kind: token.STRING, Value: "fmt.Sprintf"}, Args: args}
	case *ast.FuncLit:
		t.transpileStmts(u.Body.List)
	}
}

func (t *transpiler) transpileStmts(stmts []ast.Stmt) {
	for _, v := range stmts {
		t.transpileStmt(&v)
	}
}

func (t *transpiler) transpileStmt(stmt *ast.Stmt) {
	switch u := (*stmt).(type) {
	case *ast.DeclStmt:
		t.transpileDecl(&u.Decl)
	case *ast.ExprStmt:
		t.transpileExpr(&u.X)
	case *ast.SendStmt:
		t.transpileExpr(&u.Value)
	case *ast.AssignStmt:
		t.transpileExprs(u.Rhs)
	case *ast.GoStmt:
		t.transpileExprs(u.Call.Args)
		t.transpileExpr(&u.Call.Fun)
	case *ast.DeferStmt:
		t.transpileExprs(u.Call.Args)
		t.transpileExpr(&u.Call.Fun)
	case *ast.ReturnStmt:
		t.transpileExprs(u.Results)
	case *ast.BlockStmt:
		t.transpileStmts(u.List)
	case *ast.IfStmt:
		t.transpileStmt(&u.Init)
		t.transpileExpr(&u.Cond)
		t.transpileStmt(&u.Else)
		t.transpileStmts(u.Body.List)
	case *ast.SwitchStmt:
		t.transpileStmt(&u.Init)
		t.transpileExpr(&u.Tag)
		t.transpileStmts(u.Body.List)
	case *ast.CaseClause:
		t.transpileExprs(u.List)
		t.transpileStmts(u.Body)
	case *ast.TypeSwitchStmt:
		t.transpileStmt(&u.Init)
		t.transpileStmt(&u.Assign)
		t.transpileStmts(u.Body.List)
	case *ast.SelectStmt:
		t.transpileStmts(u.Body.List)
	case *ast.ForStmt:
		t.transpileStmt(&u.Init)
		t.transpileExpr(&u.Cond)
		t.transpileStmts(u.Body.List)
		t.transpileStmt(&u.Post)
	case *ast.RangeStmt:
		t.transpileStmts(u.Body.List)
	}

}

func processFile(filename string, in io.Reader, out io.Writer, stdin bool) error {
	if in == nil {
		f, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer f.Close()
		in = f
	}

	src, err := ioutil.ReadAll(in)
	if err != nil {
		return err
	}

	file, err := parser.ParseFile(fileSet, filename, src, parser.ParseComments)
	if err != nil {
		return err
	}

	t := &transpiler{file}

	for _, d := range file.Decls {
		t.transpileDecl(&d)
	}

	cfg := printer.Config{Mode: printer.UseSpaces, Tabwidth: 4}

	if err = cfg.Fprint(out, fileSet, file); err != nil {
		return err
	}

	return nil
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if err := processFile("testdata/main.gpp", nil, os.Stdout, false); err != nil {
		fmt.Fprintf(os.Stderr, "could not open file: %s", err)
	}
	os.Exit(0)
}
