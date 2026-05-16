package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"jjw": main,
	})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(env *testscript.Env) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			bin := filepath.Join(wd, "testdata", "bin")
			path := bin + string(os.PathListSeparator) + env.Getenv("PATH")
			if runtime.GOOS == "windows" {
				path = bin + ";" + env.Getenv("PATH")
			}
			env.Setenv("PATH", path)
			return nil
		},
	})
}
