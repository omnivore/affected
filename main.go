package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var ignoreDirs []string

func main() {
	d := flag.String("ignore-dirs", ".checkout_git", "Dir patterns (static string) to ignore, comma-separated")
	flag.Parse()
	if len(*d) > 0 {
		ignoreDirs = strings.Split(*d, ",")
	}

	args := flag.Args()
	if len(args) != 1 {
		die("Usage: %s commit..commit\n", os.Args[0])
	}

	commitRange := args[0]
	files := changedFiles(commitRange)
	module := currentModule()
	pkgsToDeps := packagePathsToDeps()

	editedPackages := make(map[string]bool)
	for _, f := range files {
		if isIgnored(f) {
			continue
		}
		editedPackages[filepath.Dir(filepath.Join(module, f))] = true
	}

	affectedPackages := make(map[string]bool)
	for pkg, deps := range pkgsToDeps {
		// Was this package itself modified?
		if _, ok := editedPackages[pkg]; ok {
			affectedPackages[pkg] = true
		}

		// Were any of this package's recursive dependencies modified?
		for _, dep := range deps {
			if _, ok := editedPackages[dep]; ok {
				affectedPackages[pkg] = true
			}
		}
	}

	var affectedPackageList []string
	for pkg := range affectedPackages {
		affectedPackageList = append(affectedPackageList, pkg)
	}
	sort.Strings(affectedPackageList)
	fmt.Println(strings.Join(affectedPackageList, "\n"))
}

func die(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

func isIgnored(f string) bool {
	for _, d := range ignoreDirs {
		if strings.Contains(f, d) {
			return true
		}
	}
	return false
}

func currentModule() string {
	cmd := exec.Command("go", "list", "-m")
	dat, err := cmd.Output()
	if err != nil {
		die("Could not run git go list -m: %v", err)
	}

	return strings.TrimSpace(string(dat))
}

func packagePathsToDeps() map[string][]string {
	cmd := exec.Command("go", "list", "-f", "{{ .ImportPath}} {{ .Deps }}", "./...")
	dat, err := cmd.Output()
	if err != nil {
		die("Could not find git root: %s", err)
	}

	var result = make(map[string][]string)
	datString := string(dat)
	for _, pkgLine := range strings.Split(datString, "\n") {
		stringParts := strings.SplitN(pkgLine, " ", 2)
		importPath := stringParts[0]
		if len(stringParts) == 2 {
			result[importPath] = strings.Split(strings.Trim(stringParts[1], "[]"), " ")
		}
	}
	return result
}

func changedFiles(commitRange string) []string {
	cmd := exec.Command("git", "diff", "--name-only", commitRange)
	dat, err := cmd.Output()
	if err != nil {
		die("Could not run git diff-tree: %v", err)
	}
	files := strings.Split(string(dat), "\n")
	var res []string
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if !strings.HasSuffix(f, ".go") {
			// skip non-Go files
			continue
		}
		res = append(res, f)
	}
	return res
}
