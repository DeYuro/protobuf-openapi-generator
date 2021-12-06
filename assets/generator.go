package main

import (
	"bufio"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

const optionTemplate = "\noption go_package = \"%s\";\n"

func main() {
	if err := app(); err != nil {
		log.Fatal("regenerate-proto failed with error: ", err)
	}
}

func app() error {
	err := CopyDirectory("/input", "/generator")
	protoMap, err := getProtoFiles("/generator")
	if err != nil {
		return err
	}

	var pds []ProtoDeclaration
	for _, protoFiles := range protoMap {
		pd, err := NewProtoDeclaration(protoFiles)

		if err != nil {
			return err
		}
		modifyFiles(pd)

		pds = append(pds, pd)
	}

	for _, pd := range pds {
		for _, file := range pd.Files {
			err = generate(file, pd.Folder)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func generate(filePath, importPath string) error {

	outputFile := sourceRelative(filePath)
	outputDir := path.Dir(outputFile)
	err := CreateIfNotExists(outputDir, 0755)

	if err != nil {
		return err
	}

	cmd := exec.Command("protoc",
		append([]string{"-I/usr/local/include",
			"-I" + importPath,
			"--openapi_out=" + outputDir}, filePath)...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return err
	}
	err = renameFile(path.Join(outputDir, "openapi.yaml"), outputFile)
	if err != nil {
		return err
	}

	err = removeWithoutTitle(outputFile)
	if err != nil {
		return err
	}

	return err
}

func removeWithoutTitle(outputFile string) error {
	type openApi struct {
		Info    struct {
			Title   string `yaml:"title"`
			Version string `yaml:"version"`
		}
	}
	var openapi openApi
	yamlFile, err := ioutil.ReadFile(outputFile)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(yamlFile, &openapi)

	if openapi.Info.Title == "" {
		_ = os.Remove(outputFile)
	}
	return err
}

func renameFile(generatedFile, outputFile string) error {
	if _, err := os.Stat(path.Join(generatedFile, "openapi.yaml")); errors.Is(err, os.ErrNotExist) {
		return err
	}

	err := os.Rename(generatedFile, outputFile)
	return err
}

//sourceRelative unfortunately plugin don`t have opt `paths=source relative`
func sourceRelative(filePath string) string {
	return strings.Replace(strings.Replace(filePath, "tmp", "output", 1), ".proto", ".yaml", 1)
}

type (
	protoFolder = string
	protoFile   = string

	ProtoDeclaration struct {
		PackageName string
		Folder      string
		Files       []string
	}
)

func NewProtoDeclaration(files []string) (ProtoDeclaration, error) {
	if len(files) == 0 {
		return ProtoDeclaration{}, io.ErrUnexpectedEOF
	}

	packageName, folder := getPackageNameAndFolder(files[0])
	return ProtoDeclaration{
		PackageName: packageName,
		Folder:      folder,
		Files:       files,
	}, nil
}

func getPackageNameAndFolder(filename string) (string, string) {
	path := filepath.Dir(filename)
	pathParts := strings.Split(path, string(filepath.Separator))
	for i := len(pathParts) - 1; i != 0; i-- {
		if i+1 < len(pathParts) && pathParts[i+1] == "proto" {
			return strings.Join(pathParts[i:], "_"), strings.Join(pathParts[:i], "/")
		}
	}

	panic("failed to get top level directory for proto files")
}

func getProtoFiles(dir string) (map[protoFolder][]protoFile, error) {
	protoPaths := map[protoFolder][]protoFile{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if info.Name() == "vendor" {
				return filepath.SkipDir
			}

			matches, err := filepath.Glob(filepath.Join(path, "*.proto"))
			if err != nil {
				return err
			} else if len(matches) != 0 {
				protoPaths[path] = matches
			}
		}

		return err
	})
	return protoPaths, err
}

func modifyFiles(pd ProtoDeclaration) {
	for _, file := range pd.Files {
		addPackageOption(file, pd.PackageName)
	}
}

func addPackageOption(file protoFile, packageName string) {

	f, err := os.OpenFile(file, os.O_APPEND|os.O_RDWR, 0644)

	if err != nil {
		panic(err)
	}
	defer f.Close()

	if isOptionExist(f) {

		return
	}

	if _, err := f.WriteString(fmt.Sprintf(optionTemplate, "/"+packageName)); err != nil {
		panic(err)
	}
}

func isOptionExist(f *os.File) bool {

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "option go_package") {
			return true
		}
	}

	return false
}

//CopyDirectory func for prevent to use third side packages
func CopyDirectory(scrDir, dest string) error {
	entries, err := ioutil.ReadDir(scrDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(scrDir, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		fileInfo, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}

		stat, ok := fileInfo.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("failed to get raw syscall.Stat_t data for '%s'", sourcePath)
		}

		switch fileInfo.Mode() & os.ModeType {
		case os.ModeDir:
			if err := CreateIfNotExists(destPath, 0755); err != nil {
				return err
			}
			if err := CopyDirectory(sourcePath, destPath); err != nil {
				return err
			}
		case os.ModeSymlink:
			if err := CopySymLink(sourcePath, destPath); err != nil {
				return err
			}
		default:
			if err := Copy(sourcePath, destPath); err != nil {
				return err
			}
		}

		if err := os.Lchown(destPath, int(stat.Uid), int(stat.Gid)); err != nil {
			return err
		}

		isSymlink := entry.Mode()&os.ModeSymlink != 0
		if !isSymlink {
			if err := os.Chmod(destPath, entry.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

func Copy(srcFile, dstFile string) error {
	out, err := os.Create(dstFile)
	if err != nil {
		return err
	}

	defer out.Close()

	in, err := os.Open(srcFile)
	defer in.Close()
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return nil
}

func Exists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	return true
}

func CreateIfNotExists(dir string, perm os.FileMode) error {
	if Exists(dir) {
		return nil
	}

	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("failed to create directory: '%s', error: '%s'", dir, err.Error())
	}

	return nil
}

func CopySymLink(source, dest string) error {
	link, err := os.Readlink(source)
	if err != nil {
		return err
	}
	return os.Symlink(link, dest)
}
