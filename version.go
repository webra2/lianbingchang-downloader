package main

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path"
)

func checkVersion(cdUrl string, subPath string) error {
	resp, err := http.Get(cdUrl + "/old/versions.json")
	if err != nil {
		return err
	}

	nowVersion, err := os.ReadFile(path.Join(subPath, "VERSION"))
	if err != nil {
		os.Create(path.Join(subPath, "VERSION"))
		err = nil
	}

	newVersion, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// version same return
	if bytes.Compare(nowVersion, newVersion) == 0 {
		os.Exit(0)
	}
	return os.WriteFile(path.Join(subPath, "VERSION"), newVersion, 0777)
}

func updateServiceINI(cdUrl string, subPath string) error {
	resp, err := http.Get(cdUrl + "/servers.ini")
	if err != nil {
		return err
	}

	nowVersion, err := os.ReadFile(path.Join(subPath, ""))
	if err != nil {
		os.Create(path.Join(subPath, "servers.ini"))
		err = nil
	}

	newVersion, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// services.ini same return
	if bytes.Compare(nowVersion, newVersion) == 0 {
		return nil
	}
	return os.WriteFile(path.Join(subPath, "servers.ini"), newVersion, 0777)
}
