package initrd

import (
	"bytes"
	"compress/gzip"
	"embed"
	"io"
	"io/fs"
	"strings"

	"github.com/cavaliergopher/cpio"
)

//go:embed rootfs/*
var GuestContent embed.FS

var Initrd []byte

func init() {
	var out bytes.Buffer

	readInitrd(&out)

	Initrd = out.Bytes()
}

func readInitrd(out io.Writer) {
	gz := gzip.NewWriter(out)
	defer gz.Close()

	w := cpio.NewWriter(gz)
	defer w.Close()

	err := fs.WalkDir(GuestContent, "rootfs", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel := strings.TrimPrefix(p, "rootfs")
		if rel == "" {
			return nil
		}
		rel = strings.TrimPrefix(rel, "/")

		if d.IsDir() {
			return w.WriteHeader(&cpio.Header{
				Name: rel,
				Mode: cpio.TypeDir | 0755,
				Size: 0,
			})
		}

		data, err := fs.ReadFile(GuestContent, p)
		if err != nil {
			return err
		}

		if err := w.WriteHeader(&cpio.Header{
			Name: rel,
			Mode: 0755,
			Size: int64(len(data)),
		}); err != nil {
			return err
		}

		_, err = w.Write(data)
		return err
	})

	if err != nil {
		panic(err)
	}
}
