// Command pw2 provides the pw2 password manager HTTP service.
//
// pw2 stores passwords encrypted with GPG via StackExchange's blackbox
// tool, backed by git. Pw2 requires libgit2 and blackbox
// (https://github.com/StackExchange/blackbox) to be installed to function.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"reflect"

	"github.com/Zemnmez/pw2/pkg/pw2"
	"github.com/golang/glog"
	"github.com/libgit2/git2go"
)

const repoDir = "git"

var httpAddr string

func init() {
	flag.StringVar(
		&httpAddr,
		"http",
		":http",
		`The address to bind the http server to eg: ":http", "127.0.0.1:8080"`,
	)
}

func do() (err error) {
	var db pw2.Database
	if db, err = pw2.Open(repoDir); err != nil {
		if pw2.DatabaseNotFound(err) {
			glog.Info("Database did not exist, creating...")
			db, err = pw2.Create(repoDir)
		}
		return
	}

	_ = db

	return
}

func main() {
	flag.Parse()

	if err := do(); err != nil {
		var buf bytes.Buffer
		switch v := err.(type) {
		case *git.GitError:
			var strCode, strClass string

			var ok bool
			if strCode, ok = gitErrorClassNames[v.Class]; !ok {
				strCode = fmt.Sprintf("#%v", v.Class)
			}

			if strClass, ok = gitErrorCodeNames[v.Code]; !ok {
				strClass = fmt.Sprintf("#%v", v.Code)
			}

			fmt.Fprintf(
				&buf,
				`Fatal git error:
	Code: %s
	Class: %s
	Message: %s`,
				strCode,
				strClass,
				v.Message,
			)

		default:
			fmt.Fprintf(&buf, "Fatal error: %+q %s", err, reflect.TypeOf(err))
		}
		glog.Fatal(buf.String())
	}

	return
}
