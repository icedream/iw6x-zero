package main

import (
	"debug/pe"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type fileSystem struct {
	baseDir string
}

func (fs *fileSystem) FullPath(name string) string {
	return filepath.Join(fs.baseDir, name)
}

func (fs *fileSystem) MkdirAll(name string, perm os.FileMode) (err error) {
	return os.MkdirAll(fs.FullPath(name), perm)
}

func (fs *fileSystem) Readlink(name string) (string, error) {
	return os.Readlink(fs.FullPath(name))
}

func (fs *fileSystem) SymlinkFromFs(name string, targetFs *fileSystem) (err error) {
	sourcePath := fs.FullPath(name)
	targetPath := targetFs.FullPath(name)
	return os.Symlink(sourcePath, targetPath)
}

func (fs *fileSystem) CopyToFs(name string, targetFs *fileSystem, mode os.FileMode) (err error) {
	var sourceFile, targetFile *os.File
	sourceFile, err = fs.OpenFile(name, os.O_RDONLY, 0)
	if err != nil {
		return
	}
	defer sourceFile.Close()
	if mode == 0 {
		var sourceStat os.FileInfo
		sourceStat, err = fs.Stat(name)
		if err != nil {
			return
		}
		mode = sourceStat.Mode().Perm()
	}
	targetFile, err = targetFs.OpenFile(name, os.O_CREATE|os.O_WRONLY, mode|0100)
	if err != nil {
		return
	}
	if _, err = io.Copy(targetFile, sourceFile); err != nil {
		return
	}
	err = os.Chmod(targetFile.Name(), mode)
	return
}

func (fs *fileSystem) Open(name string) (f *os.File, err error) {
	if filepath.IsAbs(name) {
		err = fmt.Errorf("unexpected absolute path, expected relative: %s", name)
		return
	}

	f, err = os.Open(fs.FullPath(name))
	return
}

func (fs *fileSystem) Stat(name string) (f os.FileInfo, err error) {
	f, err = os.Stat(fs.FullPath(name))
	return
}

func (fs *fileSystem) Lstat(name string) (f os.FileInfo, err error) {
	f, err = os.Lstat(fs.FullPath(name))
	return
}

func (fs *fileSystem) OpenFile(name string, flag int, perm os.FileMode) (f *os.File, err error) {
	if filepath.IsAbs(name) {
		err = fmt.Errorf("unexpected absolute path, expected relative: %s", name)
		return
	}

	f, err = os.OpenFile(fs.FullPath(name), flag, perm)
	return
}

func (fs *fileSystem) Walk(walkFn filepath.WalkFunc) error {
	return filepath.Walk(fs.baseDir, func(path string, info os.FileInfo, err error) error {
		originalErr := err
		path, err = filepath.Rel(fs.baseDir, path)
		if err == nil {
			err = originalErr
		}
		return walkFn(path, info, err)
	})
}

func (fs *fileSystem) GetImportedLibrariesOfPEFile(path string) (importedLibraries []string, err error) {
	if filepath.IsAbs(path) {
		err = fmt.Errorf("unexpected absolute path, expected relative: %s", path)
		return
	}

	f, err := fs.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return
	}
	defer f.Close()
	pef, err := pe.NewFile(f)
	if err != nil {
		return
	}
	defer pef.Close()

	// NOTE - can't use pef.ImportedLibraries() because on Linux it actually does not do anything
	importedSymbols, err := pef.ImportedSymbols()
	if err != nil {
		return
	}

	importedLibraries = []string{}
symbols:
	for _, symbol := range importedSymbols {
		symbolPath := strings.SplitN(symbol, ":", 2)[1]

		// do not insert duplicate entry
		for _, includedSymbolPath := range importedLibraries {
			if strings.EqualFold(includedSymbolPath, symbolPath) {
				continue symbols
			}
		}

		// do not insert Windows-provided DLL
		for _, includedSymbolPath := range windowsFiles {
			if strings.EqualFold(includedSymbolPath, symbolPath) {
				continue symbols
			}
		}

		log.Println("Imported library:", symbolPath)
		importedLibraries = append(importedLibraries, symbolPath)
	}

	return
}

func (fs *fileSystem) CheckGlobWhitelist(requiredFiles []string) (ok bool, mismatchedPattern string, allCheckedPaths []string, err error) {
	var matchedPaths, importedLibraries, additionalCheckedPaths []string

	allCheckedPaths = []string{}

	for _, pattern := range requiredFiles {
		log.Println("Checking:", pattern)

		matchedPaths, err = filepath.Glob(fs.FullPath(filepath.FromSlash(pattern)))
		if err != nil {
			log.Fatal(err)
		}

		if len(matchedPaths) <= 0 {
			mismatchedPattern = pattern
			return
		}

		for _, matchedPath := range matchedPaths {
			var relPath string
			relPath, err = filepath.Rel(fs.baseDir, matchedPath)
			if err != nil {
				return
			}
			log.Println("Found:", relPath)
			allCheckedPaths = append(allCheckedPaths, relPath)
			switch {
			case strings.EqualFold(filepath.Ext(matchedPath), ".exe"): // *.exe
				// check if all imported library files exist
				log.Println("Checking: Imports of", relPath)
				importedLibraries, err = fs.GetImportedLibrariesOfPEFile(relPath)
				if err != nil {
					return
				}
				log.Println("Found", len(importedLibraries), "imported libraries")
				// TODO - do not actually parse list as globs, these are hard filenames!
				ok, mismatchedPattern, additionalCheckedPaths, err = fs.CheckGlobWhitelist(importedLibraries)
				if !ok || err != nil {
					return
				}
				allCheckedPaths = append(allCheckedPaths, additionalCheckedPaths...)
			}
		}
	}
	ok = true
	return
}

func newFileSystem(baseDir string) *fileSystem {
	return &fileSystem{
		baseDir: baseDir,
	}
}
