package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const cfgPath = "/run/config"

const metadataURL = "http://169.254.169.254"

const gcpURL = metadataURL + "/computeMetadata/v1/instance/"
const awsURL = metadataURL + "/latest/meta-data/"

var envrgx = regexp.MustCompile(`^([\w-_/]+),([\w-_/\.]+),([\d]{4})$`)

var INF = log.New(os.Stdout, "metadata [INF] ", 0)
var ERR = log.New(os.Stdout, "metadata [ERR] ", 0)

var client = &http.Client{
	Timeout: time.Second * 2,
}

type header struct {
	name  string
	value string
}

var gcpHeaders = []header{
	{
		name:  "Metadata-Flavor",
		value: "Google",
	},
}

var awsHeaders = []header{}

type attribute struct {
	key  string
	file string
	mode uint
}

var gcpAttributes = []attribute{
	{
		key:  "hostname",
		file: "hostname",
		mode: 0644,
	},
	{
		key:  "name",
		file: "local_hostname",
		mode: 0644,
	},
	{
		key:  "attributes/ssh-keys",
		file: "ssh/authorized_keys",
		mode: 0600,
	},
	{
		key:  "attributes/user-data",
		file: "userdata",
		mode: 0644,
	},
}

var awsAttributes = []attribute{
	{
		key:  "instance-id",
		file: "instance_id",
		mode: 0644,
	},
	{
		key:  "local-ipv4",
		file: "local_ipv4",
		mode: 0644,
	},
	{
		key:  "public-ipv4",
		file: "public_ipv4",
		mode: 0644,
	},
	{
		key:  "public-ipv4",
		file: "public_ipv4",
		mode: 0644,
	},
}

func get(url, key, file string, mode uint, headers []header) {
	fullURL := url + key

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		ERR.Printf("unable to create request: %v", err)
		return
	}

	for _, h := range headers {
		req.Header.Set(h.name, h.value)
	}

	INF.Printf("retrieving %s", fullURL)

	resp, err := client.Do(req)
	if err != nil {
		ERR.Printf("request failed: %v", err)
		return
	}
	if resp.StatusCode != 200 {
		ERR.Printf("unexpected status code: %d", resp.StatusCode)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ERR.Printf("unable to read response: %v", err)
		return
	}

	dir := path.Dir(file)
	if dir != "." {
		if err := os.MkdirAll(dir, os.ModeDir|0775); err != nil {
			ERR.Printf("unable to create directory %s: %v", dir, err)
			return
		}
	}

	if err := ioutil.WriteFile(file, body, os.FileMode(mode)); err != nil {
		ERR.Printf("unable to write file: %v", err)
		return
	}
}

func gcpGet(key, file string, mode uint) {
	INF.Printf("GCP: get %s -> %s", key, file)
	get(gcpURL, key, file, mode, gcpHeaders)
}

func awsGet(key, file string, mode uint) {
	INF.Printf("AWS: get %s -> %s", key, file)
	get(awsURL, key, file, mode, awsHeaders)
}

func fromEnv(prefix string) []attribute {
	attrs := []attribute{}
	for _, e := range os.Environ() {
		kv := strings.SplitN(e, "=", 2)
		if !strings.HasPrefix(kv[0], prefix) {
			continue
		}
		m := envrgx.FindStringSubmatch(kv[1])
		if len(m) < 1 {
			continue
		}

		mode, err := strconv.ParseUint(m[3], 8, 32)
		if err != nil {
			ERR.Printf("unable to parse mode for env %s: %v", e, err)
		}

		attrs = append(attrs, attribute{
			key:  m[1],
			file: m[2],
			mode: uint(mode),
		})
	}
	return attrs
}

func main() {
	if err := os.MkdirAll(cfgPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "unable to create config dir %s: %v\n", cfgPath, err)
		os.Exit(1)
	}

	if err := os.Chdir(cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "unable to change current dir %s: %v\n", cfgPath, err)
		os.Exit(1)
	}

	gcpAttributes = append(gcpAttributes, fromEnv("GCP_")...)
	for _, a := range gcpAttributes {
		gcpGet(a.key, a.file, a.mode)
	}

	awsAttributes = append(awsAttributes, fromEnv("AWS_")...)
	for _, a := range awsAttributes {
		awsGet(a.key, a.file, a.mode)
	}
}
