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
var onlyGoFiles bool

func main() {
	d := flag.String("ignore-dirs", ".checkout_git", "Dir patterns (static string) to ignore, comma-separated")
	o := flag.String("only-go", "false", "Any value to set this flag to true")

	flag.Parse()
	if len(*d) > 0 {
		ignoreDirs = strings.Split(*d, ",")
	}
	if *o == "true" {
		onlyGoFiles = true
	}

	var commitRange string
	args := flag.Args()
	if len(args) >= 1 {
		commitRange = args[0]
	}

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
	if onlyGoFiles {
		for pkg := range affectedPackages {
			affectedPackageList = append(affectedPackageList, pkg)
		}
	} else {
		for pkg := range editedPackages {
			affectedPackageList = append(affectedPackageList, pkg)
		}
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
		die("Could not run git go list -m: %v", stdErrMsg(err))
	}

	return strings.TrimSpace(string(dat))
}

func packagePathsToDeps() map[string][]string {
	cmd := exec.Command("go", "list", "-f", "{{ .ImportPath}} {{ .Deps }}", "./...")
	dat, err := cmd.Output()
	if err != nil {
		die("Could not find git root: %s", stdErrMsg(err))
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
	var cmd *exec.Cmd
	var err error
	var diffTree, localCached, local []byte

	if commitRange != "" {
		cmd = exec.Command("git", "diff-tree", "--relative", "--no-commit-id", "--name-only", "-r", commitRange)
		diffTree, err = cmd.Output()
		if err != nil {
			die("Could not run git diff-tree: %v", stdErrMsg(err))
		}
	}

	cmd = exec.Command("git", "diff", "--cached", "--name-only")
	localCached, err = cmd.Output()
	if err != nil {
		die("Could not run git diff --cached --name-only: %v", stdErrMsg(err))
	}
	cmd = exec.Command("git", "diff", "--name-only")
	local, err = cmd.Output()
	if err != nil {
		die("Could not run git diff --name-only: %v", stdErrMsg(err))
	}

	changed := string(diffTree) + string(localCached) + string(local)
	files := strings.Split(changed, "\n")
	var res []string
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}

		res = append(res, f)
	}
	return res
}

func stdErrMsg(err error) string {
	if stderr, ok := err.(*exec.ExitError); ok {
		return strings.TrimSpace(string(stderr.Stderr))
	}
	return err.Error()
}
