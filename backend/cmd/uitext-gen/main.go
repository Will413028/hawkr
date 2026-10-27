// Command uitext-gen 由 buf image 與共用文案目錄的來源檔生成 Go、TS、Dart 的執行期程式碼、next-intl 的 messages、
// 共用 golden 與守門用的 manifest；驗證不過就不輸出（設計 step10e-Lprime-design.md §2）。
//
// 用法（在 backend 目錄）：
//
//	buf build ../api -o -#format=binpb | GOWORK=off go run ./cmd/uitext-gen -image - -root .. -write
//
// -write 同時把來源檔寫成正規形式（key 排序、description 由 proto 註解同步）。這是開發工具，不部署。
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "uitext-gen:", err)
		os.Exit(1)
	}
}

func run() error {
	image := flag.String("image", "-", "buf image（binpb）的路徑，- 是標準輸入")
	root := flag.String("root", "..", "repo 根目錄")
	write := flag.Bool("write", false, "把來源檔寫成正規形式")
	flag.Parse()

	var data []byte
	var err error
	if *image == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(*image)
	}
	if err != nil {
		return err
	}
	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(data, &fds); err != nil {
		return fmt.Errorf("讀不懂 image：%w", err)
	}
	in, err := loadInputs(*root, &fds)
	if err != nil {
		return err
	}
	in.EnforceUnavailableShape = true
	m, err := Build(in)
	if err != nil {
		return err
	}
	outputs, err := m.Outputs()
	if err != nil {
		return err
	}
	if *write {
		sources, err := m.SourceFiles()
		if err != nil {
			return err
		}
		for p, b := range sources {
			outputs[p] = b
		}
	}
	var paths []string
	for p := range outputs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if err := writeIfChanged(filepath.Join(*root, p), outputs[p]); err != nil {
			return err
		}
	}
	for _, line := range m.Coverage {
		fmt.Fprintln(os.Stderr, "覆蓋率", line)
	}
	return nil
}

func writeIfChanged(path string, content []byte) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, content) { //nolint:gosec // 生成器寫入 repo 內的固定路徑
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // 生成的原始碼目錄要能被一般使用者走訪
		return err
	}
	return os.WriteFile(path, content, 0o644) //nolint:gosec // 生成的原始碼本來就是可讀的
}

func loadInputs(root string, fds *descriptorpb.FileDescriptorSet) (Inputs, error) {
	in := Inputs{Image: fds, Catalog: map[string]*File{}}
	cfgData, err := os.ReadFile(filepath.Join(root, catalogDir, "config.json")) //nolint:gosec // 路徑由 -root 與固定的目錄組成
	if err != nil {
		return in, err
	}
	if in.Config, err = ParseConfig(cfgData); err != nil {
		return in, fmt.Errorf("config.json：%w", err)
	}
	for _, loc := range in.Config.Locales {
		data, err := os.ReadFile(filepath.Join(root, catalogDir, loc+".json")) //nolint:gosec // 路徑由 -root 與 config.json 的語系組成
		if err != nil {
			return in, err
		}
		if in.Catalog[loc], err = ParseFile(data); err != nil {
			return in, fmt.Errorf("%s.json：%w", loc, err)
		}
	}
	dirs, err := filepath.Glob(filepath.Join(root, "backend", "internal", "modules", "*", "internal", "text"))
	if err != nil {
		return in, err
	}
	sort.Strings(dirs)
	for _, d := range dirs {
		mi := ModuleInput{Dir: filepath.ToSlash(strings.TrimPrefix(d, filepath.Clean(root)+string(filepath.Separator))), Files: map[string]*File{}}
		entries, err := os.ReadDir(d)
		if err != nil {
			return in, err
		}
		for _, e := range entries {
			name, ok := strings.CutSuffix(e.Name(), ".json")
			if !ok {
				continue
			}
			data, err := os.ReadFile(filepath.Join(d, e.Name())) //nolint:gosec // 路徑是模組 text/ 目錄裡的來源檔
			if err != nil {
				return in, err
			}
			if mi.Files[name], err = ParseFile(data); err != nil {
				return in, fmt.Errorf("%s/%s：%w", mi.Dir, e.Name(), err)
			}
		}
		if len(mi.Files) > 0 {
			in.Modules = append(in.Modules, mi)
		}
	}
	return in, nil
}
