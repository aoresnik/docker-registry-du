package main

// NOTE: github.com/nokia/docker-registry-client/registry is used only to read the v2/_catalog
// go-containerregistry is used for all other operations (it doesn't have support for catalog)

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"syscall"

	"github.com/nokia/docker-registry-client/registry"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const APP_VERSION = "0.1alpha"

type LayerData struct {
	used_by_tags   map[*TagData]bool
	used_by_images map[*ImageData]bool
	size           int64
}

type TagData struct {
	layers map[*LayerData]bool
	name   string

	// total sum of layers size
	size int64
}

type ImageData struct {
	tags map[*TagData]bool
	name string
}

type RepoData struct {
	images map[*ImageData]bool

	// Map by digest
	all_layers map[string]LayerData
}

// The flag package provides a default help printer via -h switch
var versionFlag *bool = flag.Bool("v", false, "Print the version number.")

var username *string = flag.String("username", "", "Username for registry")

var password *string = flag.String("password", "", "Password for registry (NOTE: passing password via parameter is insecure)")

var askPassword *bool = flag.Bool("p", false, "Ask for password")

func readRepoData_gocontainerregistry(hub *registry.Registry, repositories []string, rawUrl string, username string, password string) *RepoData {
	parsed, err := url.Parse(rawUrl)
	if err != nil {
		log.Fatal(err)
	}
	repoPrefix := fmt.Sprintf("%s", parsed.Host /*, parsed.Port()*/)

	// Also works for JWT
	auth := &authn.Basic{
		Username: username,
		Password: password,
	}

	repoData := new(RepoData)
	repoData.images = make(map[*ImageData]bool)

	layersByDigest := make(map[string]*LayerData)

	for _, repo := range repositories {
		fmt.Printf("Reading data for image: %s\n", repo)
		ref, err := name.NewRepository(repoPrefix + "/" + repo)
		if err != nil {
			log.Default().Printf("Error parsing repo name: %v", err)
			continue
		}

		tags, err := remote.List(ref, remote.WithAuth(auth))
		if err != nil {
			// FIXME: errors should be included in the report, because the sizes are not accurate anymore
			log.Printf("Skipping the repo "+repo+" because of error while obtaining tags list: %v\n", err)
			continue
		}

		imageData := new(ImageData)
		imageData.name = repo
		imageData.tags = make(map[*TagData]bool)
		repoData.images[imageData] = true

		fmt.Printf("Image %s has %d tags\n", ref.Name(), len(tags))

		for _, tag := range tags {
			tagData := new(TagData)
			tagData.name = tag
			tagData.layers = make(map[*LayerData]bool)
			imageData.tags[tagData] = true

			tagRef := ref.Tag(tag)

			// 3. Get the image for this tag
			img, err := remote.Image(tagRef, remote.WithAuth(auth))
			if err != nil {
				// FIXME: errors should be included in the report, because the sizes are not accurate anymore
				log.Printf("Skipping the tag "+tagData.name+" because of error while obtaining image data: %v\n", err)
				continue
			}

			// 4. Get the manifest
			manifest, err := img.Manifest()
			if err == nil {
				for _, layer := range manifest.Layers {
					layerData, present := layersByDigest[layer.Digest.String()]
					if !present {
						layerData = new(LayerData)
						layerData.size = layer.Size
						layerData.used_by_tags = make(map[*TagData]bool)
						layerData.used_by_images = make(map[*ImageData]bool)

						layersByDigest[layer.Digest.String()] = layerData
					}

					layerData.used_by_tags[tagData] = true
					layerData.used_by_images[imageData] = true
					tagData.layers[layerData] = true
					tagData.size += layerData.size

				}
			} else {
				// FIXME: errors should be included in the report, because the sizes are not accurate anymore
				log.Printf("Skipping the tag "+tagData.name+" because of error while obtaining manifest: %s\n", err.Error())
			}
		}
	}
	return repoData
}

func SizeInMiB(size int64) int64 {
	return size / (1024 * 1024)
}

func repoDataPrintReport(repoData *RepoData) {
	for imageData, _ := range repoData.images {
		var nImageSharedSize int64
		var nImageExclusiveSize int64
		var nImageSize int64
		var nImageSharedLayers int
		var nImageExclusiveLayers int
		var nImageLayers int
		countedLayers := make(map[*LayerData]bool)
		for tagData, _ := range imageData.tags {
			for layerData, _ := range tagData.layers {
				if !countedLayers[layerData] {
					nImageLayers++
					nImageSize += layerData.size
					if len(layerData.used_by_images) > 1 {
						nImageSharedLayers++
						nImageSharedSize += layerData.size
					} else {
						nImageExclusiveLayers++
						nImageExclusiveSize += layerData.size
					}
					countedLayers[layerData] = true
				}
			}
		}
		fmt.Printf("Image %s: total %d MiB (%d layers), shared %d MiB (%d layers), exclusive %d MiB (%d layers)\n", imageData.name, SizeInMiB(nImageSize), nImageLayers, SizeInMiB(nImageSharedSize), nImageSharedLayers, SizeInMiB(nImageExclusiveSize), nImageExclusiveLayers)
		for tagData, _ := range imageData.tags {
			var nSharedSize int64
			var nExclusiveSize int64
			var nSize int64
			var nSharedLayers int
			var nExclusiveLayers int
			var nLayers int
			for layerData, _ := range tagData.layers {
				nLayers++
				nSize += layerData.size
				if len(layerData.used_by_tags) > 1 {
					nSharedLayers++
					nSharedSize += layerData.size
				} else {
					nExclusiveLayers++
					nExclusiveSize += layerData.size
				}
			}
			fmt.Printf("  Tag %s: total %d MiB (%d layers), shared %d MiB (%d layers), exclusive %d MiB (%d layers)\n", tagData.name, SizeInMiB(nSize), nLayers, SizeInMiB(nSharedSize), nSharedLayers, SizeInMiB(nExclusiveSize), nExclusiveLayers)
		}
	}
}

func PrintUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s registry_url [repo ...]\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = PrintUsage
	flag.Parse()

	if *versionFlag {
		fmt.Println("Version:", APP_VERSION)
		return
	}

	if flag.NArg() > 0 {
		url := flag.Arg(0)
		fmt.Println("Registry: ", url)
		fmt.Println("Username: ", *username)

		var usePassword string
		if *askPassword {
			fmt.Print("Password: ")
			bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
			if err == nil {
				fmt.Println("\nPassword read")
			}
			usePassword = string(bytePassword)
		} else {
			usePassword = *password
		}
		hub, err := registry.New(url, *username, usePassword)
		if err != nil {
			log.Fatal(err)
		}

		var repositories []string
		if flag.NArg() > 1 {
			repositories = flag.Args()[1:]
		} else {
			fmt.Println("Obtaining the list of all available repositories ")
			repositories, err = hub.Repositories()
			if err != nil {
				log.Fatal(err)
			}
		}

		fmt.Println("Found  repositories ", repositories)
		repoData := readRepoData_gocontainerregistry(hub, repositories, url, *username, *password)
		repoDataPrintReport(repoData)
	} else {
		PrintUsage()
	}
}
