package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var src = flag.String("src", "", "Source directory to soft copy.")
var dst = flag.String("dst", "", "Destination for the copy.")

func main() {
	flag.Parse()
	if *src == "" || *dst == "" {
		fmt.Printf("Both --src and --dst must be set.\n")
		return
	}
	*src = filepath.Clean(*src)
	*dst = filepath.Clean(*dst)

	_, err := os.Stat(*src)
	if err != nil {
		fmt.Printf("Unable to stat source dir: %v\n", err)
		return
	}

	var internal_err error
	filepath.Walk(*src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if internal_err != nil {
				internal_err = err
			}
			return err
		}
		dst_path := filepath.Join(*dst, path[len(*src):])
		if info.IsDir() {
			err = os.Mkdir(dst_path, info.Mode().Perm())
			if internal_err != nil {
				internal_err = err
			}
			return err
		}
		err = os.Link(path, dst_path)
		if internal_err != nil {
			internal_err = err
		}
		return err
	})
	if internal_err != nil {
		fmt.Printf("Unable to properly link dest directory: %v\n", internal_err)
	}
}
