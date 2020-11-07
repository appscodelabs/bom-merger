/*
Copyright AppsCode Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	flag "github.com/spf13/pflag"
	"gomodules.xyz/mod"
)

var (
	dirIn  string
	dirOut string
)

func init() {
	flag.StringVar(&dirIn, "in", "", "Path to directory where BOM json files are stored")
	flag.StringVar(&dirOut, "out", "", "Path to directory where output files are stored")
}

type projectAndLicenses struct {
	Project  string    `json:"project"`
	Licenses []license `json:"licenses,omitempty"`
	Error    string    `json:"error,omitempty"`
	VCS      string    `json:"vcs,omitempty"`
}

type license struct {
	Type       string  `json:"type,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

var regBOM = map[string]projectAndLicenses{}
var regErrors = map[string]projectAndLicenses{}

func cleanupLicense(reg map[string]projectAndLicenses) error {
	for project, info := range reg {
		lics := make([]license, 0, len(info.Licenses))
		for _, lic := range info.Licenses {
			if lic.Confidence > 0.5 {
				lics = append(lics, lic)
			}
		}
		info.Licenses = lics
		reg[project] = info
	}
	return nil
}

func discoverVCS(reg map[string]projectAndLicenses) error {
	for project, info := range reg {
		vcs, err := mod.DetectVCSRoot(info.Project)
		if err != nil {
			return err
		}
		info.VCS = vcs
		reg[project] = info
	}
	return nil
}

func writeBOM(filename string, reg map[string]projectAndLicenses) error {
	bom := make([]projectAndLicenses, 0, len(reg))
	for _, key := range Keys(reg) {
		bom = append(bom, reg[key])
	}
	data, err := MarshalJson(bom)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, data, 0644)
}

func Keys(m map[string]projectAndLicenses) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func MarshalJson(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(v)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func loadBOM(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	decoder := json.NewDecoder(f)

	gooddoc := true
	for {
		var info []projectAndLicenses
		err = decoder.Decode(&info)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if gooddoc {
			for _, project := range info {
				regBOM[project.Project] = project
			}
			gooddoc = false
		} else {
			for _, project := range info {
				regErrors[project.Project] = project
			}
		}
	}
	return nil
}

func main() {
	flag.Parse()

	files, err := ioutil.ReadDir(dirIn)
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		if !f.IsDir() {
			err = loadBOM(filepath.Join(dirIn, f.Name()))
			if err != nil {
				panic(err)
			}
		}
	}

	err = cleanupLicense(regBOM)
	if err != nil {
		panic(err)
	}

	err = discoverVCS(regBOM)
	if err != nil {
		panic(err)
	}
	err = discoverVCS(regErrors)
	if err != nil {
		panic(err)
	}

	err = writeBOM(filepath.Join(dirOut, "bom.json"), regBOM)
	if err != nil {
		panic(err)
	}
	err = writeBOM(filepath.Join(dirOut, "bom_error.json"), regErrors)
	if err != nil {
		panic(err)
	}
}
