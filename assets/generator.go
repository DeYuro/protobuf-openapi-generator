package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

const optionTemplate = "\noption go_package = \"%s\";\n"

func main() {
	if err := app(); err != nil {
		log.Fatal("regenerate-proto failed with error: ", err)
	}
}

func app() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	protoMap, err := getProtoFiles(path.Join(wd, "api"))
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

	// Create folders
	packageFolder := filepath.Join("internal", "grpc")
	if err := os.RemoveAll(packageFolder); err != nil {
		return err
	}

	if err := os.MkdirAll(packageFolder, os.ModePerm); err != nil {
		return err
	}
	// -------

	for _, pd := range pds {
		//protocString := "--go_out=import_path=" + pd.PackageName + ","
		//for _, filename := range pd.Files {
		//	protocString += "M" + filename + "=" + pd.PackageName + ","
		//}

		//protocString := " --go_out=generated, --go-grpc_out=generated,"
		//hz := append([]string{"-I/usr/local/include", "-I" + pd.Folder, protocString}, pd.Files...)
		//_ = hz
		cmd := exec.Command("protoc",
			append([]string{"-I/usr/local/include", "-I" + pd.Folder, "--go_out=" + pd.Folder + "/generated", "--go-grpc_out=" + pd.Folder + "/generated"}, pd.Files...)...)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		err := cmd.Run()
		if err != nil {
			return err
		}
	}

	return nil
}

func ps(arg ...string)  {

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

func addPackageOption(file protoFile, packageName string)  {

	f, err := os.OpenFile(file,os.O_APPEND|os.O_RDWR, 0644)

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