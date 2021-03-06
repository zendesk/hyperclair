package docker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/zendesk/hyperclair/docker/httpclient"
	"github.com/zendesk/hyperclair/xerrors"
)

//Pull Image from Registry or Hub depending on image name
func Pull(imageName string, insecure bool) (Image, error) {
	image, err := Parse(imageName, insecure)
	if err != nil {
		return Image{}, err
	}

	logrus.Info("pulling image: ", image)

	mURI := fmt.Sprintf("%v/%v/manifests/%v", image.Registry, image.Name, image.Tag)
	client := httpclient.Get()
	request, err := http.NewRequest("GET", mURI, nil)
	response, err := client.Do(request)
	if err != nil {
		return Image{}, fmt.Errorf("retrieving manifest: %v", err)
	}

	if response.StatusCode == http.StatusUnauthorized {
		logrus.Info("Pull is Unauthorized")
		err := AuthenticateResponse(response, request)

		if err != nil {
			return Image{}, fmt.Errorf("authenticating: %v", err)
		}
		response, err = client.Do(request)
		if err != nil {
			return Image{}, fmt.Errorf("retrieving manifest: %v", err)
		}
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return Image{}, fmt.Errorf("reading manifest body: %v", err)
	}
	if response.StatusCode != 200 {
		switch response.StatusCode {
		case http.StatusUnauthorized:
			return Image{}, xerrors.Unauthorized
		case http.StatusNotFound:
			return Image{}, xerrors.NotFound
		default:
			return Image{}, fmt.Errorf("%d - %s", response.StatusCode, string(body))
		}
	}
	if err := image.parseManifest(body); err != nil {
		return Image{}, fmt.Errorf("parsing manifest: %v", err)
	}

	return image, nil
}

func (image *Image) parseManifest(body []byte) error {

	err := json.Unmarshal(body, &image)

	if err != nil {
		return fmt.Errorf("unmarshalling manifest body: %v", err)
	}

	image.uniqueLayers()
	return nil
}

func (image *Image) uniqueLayers() {
	encountered := map[Layer]bool{}
	result := []Layer{}
	if image.SchemaVersion == 1 {
		for index := range image.FsLayers {
			if encountered[image.FsLayers[index]] != true {
				encountered[image.FsLayers[index]] = true
				result = append(result, image.FsLayers[index])
			}
		}
		image.FsLayers = result
	} else {
		for index := range image.Layers {
			if encountered[image.Layers[index]] != true {
				encountered[image.Layers[index]] = true
				result = append(result, image.Layers[index])
			}
		}
		image.Layers = result
	}
}
