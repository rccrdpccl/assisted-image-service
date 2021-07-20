package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"

	"github.com/carbonin/assisted-image-service/pkg/imagestore"
	"github.com/kelseyhightower/envconfig"
)

var Options struct {
	AssistedServiceURL string                       `envconfig:"ASSISTED_SERVICE_URL" default:"http://assisted-service:8080"`
	Versions           map[string]map[string]string `envconfig:"RHCOS_VERSIONS"`
}

var DefaultVersions = map[string]map[string]string{
	"4.6": {
		"iso_url":    "https://mirror.openshift.com/pub/openshift-v4/dependencies/rhcos/4.6/4.6.8/rhcos-4.6.8-x86_64-live.x86_64.iso",
		"rootfs_url": "https://mirror.openshift.com/pub/openshift-v4/dependencies/rhcos/4.6/4.6.8/rhcos-live-rootfs.x86_64.img",
	},
	"4.7": {
		"iso_url":    "https://mirror.openshift.com/pub/openshift-v4/dependencies/rhcos/4.7/4.7.13/rhcos-4.7.13-x86_64-live.x86_64.iso",
		"rootfs_url": "https://mirror.openshift.com/pub/openshift-v4/dependencies/rhcos/4.7/4.7.13/rhcos-live-rootfs.x86_64.img",
	},
	"4.8": {
		"iso_url":    "https://mirror.openshift.com/pub/openshift-v4/dependencies/rhcos/pre-release/4.8.0-rc.3/rhcos-4.8.0-rc.3-x86_64-live.x86_64.iso",
		"rootfs_url": "https://mirror.openshift.com/pub/openshift-v4/dependencies/rhcos/pre-release/4.8.0-rc.3/rhcos-live-rootfs.x86_64.img",
	},
}

var clusterRegexp = regexp.MustCompile(`/images/.+`)

func parseClusterID(path string) (string, error) {
	found := clusterRegexp.FindString(path)
	if found == "" {
		return "", fmt.Errorf("malformed download path: %s", path)
	}
	return found, nil
}

func downloadImageHandler(is *imagestore.ImageStore) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		clusterID, err := parseClusterID(r.URL.Path)
		if err != nil {
			log.Printf("failed to parse cluster ID: %v\n", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		log.Printf("Get info for cluster %s here\n", clusterID)

		// TODO: Make this configurable based on returned cluster info
		f, err := is.BaseFile("4.8")
		if err != nil {
			log.Printf("Error getting base image: err: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer f.Close()

		fileInfo, err := f.Stat()
		if err != nil {
			log.Printf("Error getting file info: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		fileName := fmt.Sprintf("%s-discovery.iso", clusterID)
		http.ServeContent(w, r, fileName, fileInfo.ModTime(), f)
	}
}

func main() {
	err := envconfig.Process("cluster-image", &Options)
	if err != nil {
		log.Fatalf("Failed to process config: %v\n", err)
	}

	if Options.Versions == nil {
		Options.Versions = DefaultVersions
	}
	is, err := imagestore.NewImageStore(Options.Versions)
	if err != nil {
		log.Fatalf("Failed to create image store: %v\n", err)
	}
	err = is.Populate(context.Background())
	if err != nil {
		log.Fatalf("Failed to populate image store: %v\n", err)
	}

	http.HandleFunc("/images/", downloadImageHandler(is))

	log.Printf("Starting http handler...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}