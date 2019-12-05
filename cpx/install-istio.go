package cpx

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	repoURL                  = "https://api.github.com/repos/istio/istio/releases/19927523" //"https://api.github.com/repos/istio/istio/releases/latest"
	citrixRepoURL            = "https://github.com/citrix/citrix-istio-adaptor/archive/v1.1.0-beta.tar.gz"
	urlSuffix                = "-linux.tar.gz"
	cpxUrlSuffix             = "_linux(.*).tar.gz"
	crdPattern               = "crd(.*)yaml"
	cachePeriod              = 6 * time.Hour
	cpxGenerateYamlScriptURL = "https://raw.githubusercontent.com/citrix/citrix-istio-adaptor/master/deployment/generate_yaml.sh"
	cpxIngressGatewayURL     = "https://raw.githubusercontent.com/citrix/citrix-istio-adaptor/master/deployment/cpx-ingressgateway.tmpl"
	cpxSidecarInjectionURL   = "https://raw.githubusercontent.com/citrix/citrix-istio-adaptor/master/deployment/cpx-sidecar-injection-all-in-one.tmpl"
)

var (
	localByPassFile = "/app/istio-1.3.0.tar.gz"

	localFile           = path.Join(os.TempDir(), "istio-1.3.0.tar.gz")
	destinationFolder   = path.Join(os.TempDir(), "istio")
	basePath            = path.Join(destinationFolder, "%s")
	installFile         = path.Join(basePath, "install/kubernetes/istio-demo.yaml")
	installWithmTLSFile = path.Join(basePath, "install/kubernetes/istio-demo-auth.yaml")
	bookInfoInstallFile = path.Join(basePath, "samples/bookinfo/platform/kube/bookinfo.yaml")
	//bookInfoGatewayInstallFile = path.Join(basePath, "samples/bookinfo/networking/bookinfo-gateway.yaml")
	crdFolder = path.Join(basePath, "install/kubernetes/helm/istio-init/files/")

	localCpxIstioByPassFile       = "/app/citrix-istio-adaptor-1.1.0-beta.tar.gz"
	cpxIstioLocalFile             = path.Join(os.TempDir(), "citrix-istio-adaptor-1.1.0-beta.tar.gz")
	cpxDestinationFolder          = path.Join(os.TempDir(), "citrix-istio-adaptor-1.1.0-beta")
	cpxBasePath                   = path.Join(cpxDestinationFolder, "citrix-istio-adaptor-1.1.0-beta")
	cpxGenerateYamlScript         = path.Join(cpxBasePath, "deployment/generate_yaml.sh")
	cpxIngressGatewayFile         = path.Join(cpxBasePath, "deployment/cpx-ingressgateway.tmpl")
	cpxSidecarInjectionFile       = path.Join(cpxBasePath, "deployment/cpx-sidecar-injection-all-in-one.tmpl")
	cpxWebhookCertsScript         = path.Join(cpxBasePath, "deployment/webhook-create-signed-cert.sh")
	bookInfoCpxGatewayInstallFile = path.Join(cpxBasePath, "examples/citrix-adc-in-istio/bookinfo/deployment-yaml/bookinfo_http_gateway.yaml")
	bookInfoCpxVirtualServiceFile = path.Join(cpxBasePath, "examples/citrix-adc-in-istio/bookinfo/deployment-yaml/productpage_vs.yaml")

	defaultBookInfoDestRulesFile                 = path.Join(basePath, "samples/bookinfo/networking/destination-rule-all-mtls.yaml")
	bookInfoRouteToV1AllServicesFile             = path.Join(basePath, "samples/bookinfo/networking/virtual-service-all-v1.yaml")
	bookInfoRouteToReviewsV2ForJasonFile         = path.Join(basePath, "samples/bookinfo/networking/virtual-service-reviews-test-v2.yaml")
	bookInfoCanary50pcReviewsV3File              = path.Join(basePath, "samples/bookinfo/networking/virtual-service-reviews-50-v3.yaml")
	bookInfoCanary100pcReviewsV3File             = path.Join(basePath, "samples/bookinfo/networking/virtual-service-reviews-v3.yaml")
	bookInfoInjectDelayForRatingsForJasonFile    = path.Join(basePath, "samples/bookinfo/networking/virtual-service-ratings-test-delay.yaml")
	bookInfoInjectHTTPAbortToRatingsForJasonFile = path.Join(basePath, "samples/bookinfo/networking/virtual-service-ratings-test-abort.yaml")
)

type apiInfo struct {
	TagName    string   `json:"tag_name,omitempty"`
	PreRelease bool     `json:"prerelease,omitempty"`
	Assets     []*asset `json:"assets,omitempty"`
}

type asset struct {
	Name        string `json:"name,omitempty"`
	State       string `json:"state,omitempty"`
	DownloadURL string `json:"browser_download_url,omitempty"`
}

func (iClient *Client) getLatestReleaseURL() error {
	if iClient.cpxReleaseDownloadURL == "" || time.Since(iClient.cpxReleaseUpdatedAt) > cachePeriod {
		logrus.Debugf("API info url: %s", repoURL)
		resp, err := http.Get(repoURL)
		if err != nil {
			err = errors.Wrapf(err, "error getting latest version info")
			logrus.Error(err)
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("unable to fetch release info due to an unexpected status code: %d", resp.StatusCode)
			logrus.Error(err)
			return err
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			err = errors.Wrapf(err, "error parsing response body")
			logrus.Error(err)
			return err
		}
		// logrus.Debugf("Raw api info: %s", body)
		result := &apiInfo{}
		err = json.Unmarshal(body, result)
		if err != nil {
			err = errors.Wrapf(err, "error unmarshalling response body")
			logrus.Error(err)
			return err
		}
		logrus.Debugf("retrieved api info: %+#v", result)
		if result != nil && result.Assets != nil && len(result.Assets) > 0 {
			for _, asset := range result.Assets {
				logrus.Debugf("Asset name: %s", asset.Name)
				if strings.HasSuffix(asset.Name, urlSuffix) {
					iClient.cpxReleaseVersion = strings.Replace(asset.Name, urlSuffix, "", -1)
					iClient.cpxReleaseDownloadURL = asset.DownloadURL
					iClient.cpxReleaseUpdatedAt = time.Now()
					return nil
				}
			}
		}
		err = errors.New("unable to extract the download URL")
		logrus.Error(err)
		return err
	}
	return nil
}

func (iClient *Client) getCitrixIstioAdaptorURL() error {
	if iClient.cpxResourcesDownloadURL == "" {
		logrus.Debugf("Citrix Istio Adaptor API info url: %s", citrixRepoURL)
		//	iClient.cpxResourcesVersion = "v1.1.0-beta"
		iClient.cpxResourcesDownloadURL = citrixRepoURL
		return nil
	}
	return nil
}

func (iClient *Client) downloadFile(downloadURL, localFile string) error {
	dFile, err := os.Create(localFile)
	if err != nil {
		err = errors.Wrapf(err, "unable to create a file on the filesystem at %s", localFile)
		logrus.Error(err)
		return err
	}
	defer dFile.Close()

	logrus.Debugf("Trying to download: %s", downloadURL)
	resp, err := http.Get(downloadURL)
	if err != nil {
		err = errors.Wrapf(err, "unable to download the file from URL: %s", downloadURL)
		logrus.Error(err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("unable to download the file from URL: %s, status: %s", downloadURL, resp.Status)
		logrus.Error(err)
		return err
	}

	_, err = io.Copy(dFile, resp.Body)
	if err != nil {
		err = errors.Wrapf(err, "unable to write the downloaded file to the file system at %s", localFile)
		logrus.Error(err)
		return err
	}
	return nil
}

func (iClient *Client) untarPackage(destination, fileToUntar string) error {
	lFile, err := os.Open(fileToUntar)
	if err != nil {
		err = errors.Wrapf(err, "unable to read the local file %s", fileToUntar)
		logrus.Error(err)
		return err
	}

	gzReader, err := gzip.NewReader(lFile)
	if err != nil {
		err = errors.Wrap(err, "unable to load the file into a gz reader")
		logrus.Error(err)
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			err = errors.Wrap(err, "error during untar")
			logrus.Error(err)
			return err
		case header == nil:
			continue
		}

		fileInLoop := filepath.Join(destination, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(fileInLoop); err != nil {
				if err := os.MkdirAll(fileInLoop, 0755); err != nil {
					err = errors.Wrapf(err, "error creating directory %s", fileInLoop)
					logrus.Error(err)
					return err
				}
			}
		case tar.TypeReg:
			fileAtLoc, err := os.OpenFile(fileInLoop, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				err = errors.Wrapf(err, "error opening file %s", fileInLoop)
				logrus.Error(err)
				return err
			}

			if _, err := io.Copy(fileAtLoc, tarReader); err != nil {
				err = errors.Wrapf(err, "error writing file %s", fileInLoop)
				logrus.Error(err)
				return err
			}
			fileAtLoc.Close()
		}
	}
}

func (iClient *Client) downloadCpx() (string, error) {
	var fileName string
	_, err := os.Stat(localByPassFile)
	if err != nil {
		logrus.Debug("preparing to download the latest cpx release")
		err := iClient.getLatestReleaseURL()
		if err != nil {
			return "", err
		}
		fileName = iClient.cpxReleaseVersion
		downloadURL := iClient.cpxReleaseDownloadURL
		logrus.Debugf("retrieved latest file name: %s and download url: %s. ", fileName, downloadURL)

		proceedWithDownload := true

		lFileStat, err := os.Stat(localFile)
		if err == nil {
			if time.Since(lFileStat.ModTime()) > cachePeriod {
				proceedWithDownload = true
			} else {
				proceedWithDownload = false
			}
		}

		if proceedWithDownload {
			if err = iClient.downloadFile(downloadURL, localFile); err != nil {
				return "", err
			}
			logrus.Debug("package successfully downloaded, now unzipping . . .")
		}
	} else {
		localFile = localByPassFile
		fileName = os.Getenv("ISTIO_VERSION")
		logrus.Debugf("using local bypass file: %s & version name from env: %s.", localFile, fileName)
	}
	if err = iClient.untarPackage(destinationFolder, localFile); err != nil {
		return "", err
	}
	logrus.Debug("successfully unzipped")
	return fileName, nil
}
func (iClient *Client) downloadOtherCpxResources() (string, error) {
	_, err := os.Stat(localCpxIstioByPassFile)
	if err != nil {
		logrus.Debug("preparing to download the cpx gateway, sidecar-webhook resources")
		err = iClient.getCitrixIstioAdaptorURL()
		if err != nil {
			return "", err
		}
		cpxDownloadURL := iClient.cpxResourcesDownloadURL
		logrus.Debugf("retrieved CPX Download URL: %s", cpxDownloadURL)

		proceedWithDownload := true

		lFileStat, err := os.Stat(cpxIstioLocalFile)
		if err == nil {
			if time.Since(lFileStat.ModTime()) > cachePeriod {
				proceedWithDownload = true
			} else {
				proceedWithDownload = false
			}
		}

		if proceedWithDownload {
			if err = iClient.downloadFile(cpxDownloadURL, cpxIstioLocalFile); err != nil {
				logrus.Debug("Citrix Istio Adaptor archive could not be downloaded!")
				return "", err
			}
			logrus.Debug("package successfully downloaded, now unzipping . . .")
		}
	} else {
		cpxIstioLocalFile = localCpxIstioByPassFile
		logrus.Debugf("using local cpx bypass file: %s", cpxIstioLocalFile)
	}
	if err = iClient.untarPackage(cpxDestinationFolder, cpxIstioLocalFile); err != nil {
		logrus.Debug("Citrix Istio Adaptor archive could not be unzipped!")
		return "", err
	}
	logrus.Debug("successfully unzipped")
	return "", nil
}

func (iClient *Client) getCpxComponentYAML(fileName string) (string, error) {
	specificVersionName, err := iClient.downloadCpx()
	if err != nil {
		return "", err
	}
	installFileLoc := fmt.Sprintf(fileName, specificVersionName)
	logrus.Debugf("checking if install file exists at path: %s", installFileLoc)
	_, err = os.Stat(installFileLoc)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Error(err)
			return "", err
		}
		err = errors.Wrap(err, "unknown error")
		logrus.Error(err)
		return "", err
	}
	fileContents, err := ioutil.ReadFile(installFileLoc)
	if err != nil {
		err = errors.Wrap(err, "unable to read file")
		logrus.Error(err)
		return "", err
	}
	return string(fileContents), nil
}

func (iClient *Client) downloadFileFromURL(filepath, fileURL string) error {
	resp, err := http.Get(fileURL)
	if err != nil {
		err = errors.Wrapf(err, "error getting data from %s", fileURL)
		logrus.Error(err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		out, err := os.Create(filepath)
		if err != nil {
			err = errors.Wrapf(err, "Could not create %s file locally", filepath)
			logrus.Error(err)
			return err
		}
		defer out.Close()
		// Write response body to file
		_, err = io.Copy(out, resp.Body)
		return err
	}
	err = errors.Wrapf(err, "Call failed with response status: %s", resp.Status)
	logrus.Error(err)
	return err
}

func (iClient *Client) runGenerateYamlScript(inputTmplFile string) (string, error) {
	_, err := os.Stat(cpxGenerateYamlScript)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Error(err)
			return "", err
		}
	}
	outputYamlFile := strings.Replace(inputTmplFile, ".tmpl", ".yaml", -1)
	logrus.Debugf("Output YAML file name: %s", outputYamlFile)
	err = exec.Command("/bin/sh", cpxGenerateYamlScript, "--inputfile", inputTmplFile, "--outputfile", outputYamlFile).Run()
	if err != nil {
		logrus.Error(err)
		return "", err
	} else {
		logrus.Debugf("%s YAML generated!", outputYamlFile)
	}
	return outputYamlFile, nil
}

/*
func (iClient *Client) getCpxResourcesYamlFromURL(fileURL string) (string, error) {
	fileYamlContents, err := getFileURLContents(fileURL)
	if err != nil {
		logrus.Debugf("Could not get %s contents", fileURL)
		return "", nil
	}
	if strings.HasSuffix(fileURL, ".tmpl") {
		// Generate yaml file using generate_yaml.sh script
		installFileLoc, err = iClient.runGenerateYamlScript()
		if err != nil {
			logrus.Debugf("Could not generate YAML file from %s", fileName)
		}
	}

}
*/
func (iClient *Client) getCpxYamlContent(fileName, fileURL string) (string, error) {
	if err := iClient.downloadFileFromURL(fileName, fileURL); err != nil {
		return "", err
	}
	if strings.HasSuffix(fileName, ".tmpl") {
		// Generate yaml file using generate_yaml.sh script
		fileName, err := iClient.runGenerateYamlScript(fileName)
		if err != nil {
			logrus.Debugf("Could not generate YAML file from %s", fileName)
			return "", err
		}
	}
	fileContents, err := ioutil.ReadFile(fileName)
	if err != nil {
		err = errors.Wrap(err, "unable to read file")
		logrus.Error(err)
		return "", err
	}
	return string(fileContents), nil
}

func (iClient *Client) getCpxOtherResourcesComponentYAML(fileName string) (string, error) {
	if _, err := iClient.downloadOtherCpxResources(); err != nil {
		return "", err
	}
	installFileLoc := fileName
	logrus.Debugf("checking if install file exists at path: %s", installFileLoc)
	_, err := os.Stat(installFileLoc)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Error(err)
			return "", err
		}
		err = errors.Wrap(err, "unknown error")
		logrus.Error(err)
		return "", err
	}
	if strings.HasSuffix(fileName, ".tmpl") {
		// Generate yaml file using generate_yaml.sh script
		installFileLoc, err = iClient.runGenerateYamlScript(fileName)
		if err != nil {
			logrus.Debugf("Could not generate YAML file from %s", fileName)
		}
	}
	fileContents, err := ioutil.ReadFile(installFileLoc)
	if err != nil {
		err = errors.Wrap(err, "unable to read file")
		logrus.Error(err)
		return "", err
	}
	return string(fileContents), nil
}

func (iClient *Client) getCRDsYAML() ([]string, error) {
	res := []string{}

	rEx, err := regexp.Compile(crdPattern)
	if err != nil {
		err = errors.Wrap(err, "unable to compile crd pattern")
		logrus.Error(err)
		return nil, err
	}

	specificVersionName, err := iClient.downloadCpx()
	if err != nil {
		return nil, err
	}
	startFolder := fmt.Sprintf(crdFolder, specificVersionName)
	err = filepath.Walk(startFolder, func(currentPath string, info os.FileInfo, err error) error {
		if err == nil && rEx.MatchString(info.Name()) {
			contents, err := ioutil.ReadFile(currentPath)
			if err != nil {
				err = errors.Wrap(err, "unable to read file")
				logrus.Error(err)
				return err
			}
			res = append(res, string(contents))
		}
		return nil
	})
	if err != nil {
		err = errors.Wrap(err, "unable to read the directory")
		logrus.Error(err)
		return nil, err
	}
	return res, nil
}

func (iClient *Client) getLatestCpxYAML(installmTLS bool) (string, error) {
	var cpxYamlFileContents string
	var err error
	if installmTLS {
		cpxYamlFileContents, err = iClient.getCpxComponentYAML(installWithmTLSFile)
	} else {
		cpxYamlFileContents, err = iClient.getCpxComponentYAML(installFile)
	}
	if err != nil {
		return "", err
	}
	if _, err = iClient.downloadOtherCpxResources(); err != nil {
		logrus.Debugf("Could not download Citrix Istio Adaptor resources.")
		return "", err
	}
	cpxGatewayYaml, err := iClient.getCpxYamlContent(cpxIngressGatewayFile, cpxIngressGatewayURL)
	/*
		cpxGatewayYaml, err := iClient.getCpxOtherResourcesComponentYAML(cpxIngressGatewayFile)
	*/
	if err != nil {
		err = errors.Wrapf(err, "Could not retrieve %s", cpxIngressGatewayFile)
		logrus.Error(err)
		return "", err
	}
	cpxSidecarYaml, err := iClient.getCpxYamlContent(cpxSidecarInjectionFile, cpxSidecarInjectionURL)
	if err != nil {
		err = errors.Wrapf(err, "Could not retrieve %s", cpxSidecarInjectionFile)
		logrus.Error(err)
		return "", err
	}
	cpxYamlFileContents += cpxGatewayYaml + cpxSidecarYaml

	return cpxYamlFileContents, nil
}

func (iClient *Client) getBookInfoAppYAML() (string, error) {
	return iClient.getCpxComponentYAML(bookInfoInstallFile)
}

func (iClient *Client) getBookInfoGatewayYAML() (string, error) {
	//return iClient.getCpxComponentYAML(bookInfoGatewayInstallFile)
	gwYaml, err := iClient.getCpxOtherResourcesComponentYAML(bookInfoCpxGatewayInstallFile)
	if err != nil {
		err = errors.Wrapf(err, "Could not retrive %s", bookInfoCpxGatewayInstallFile)
		logrus.Error(err)
		return "", err
	}
	vsYaml, err := iClient.getCpxOtherResourcesComponentYAML(bookInfoCpxVirtualServiceFile)
	if err != nil {
		err = errors.Wrapf(err, "Could not retrive %s", bookInfoCpxVirtualServiceFile)
		logrus.Error(err)
		return "", err
	}
	return gwYaml + vsYaml, nil
}

func (iClient *Client) getBookInfoDefaultDesinationRulesYAML() (string, error) {
	return iClient.getCpxComponentYAML(defaultBookInfoDestRulesFile)
}

func (iClient *Client) getBookInfoRouteToV1AllServicesYAML() (string, error) {
	return iClient.getCpxComponentYAML(bookInfoRouteToV1AllServicesFile)
}

func (iClient *Client) getBookInfoRouteToReviewsV2ForJasonFile() (string, error) {
	return iClient.getCpxComponentYAML(bookInfoRouteToReviewsV2ForJasonFile)
}

func (iClient *Client) getBookInfoCanary50pcReviewsV3File() (string, error) {
	return iClient.getCpxComponentYAML(bookInfoCanary50pcReviewsV3File)
}

func (iClient *Client) getBookInfoCanary100pcReviewsV3File() (string, error) {
	return iClient.getCpxComponentYAML(bookInfoCanary100pcReviewsV3File)
}

func (iClient *Client) getBookInfoInjectDelayForRatingsForJasonFile() (string, error) {
	return iClient.getCpxComponentYAML(bookInfoInjectDelayForRatingsForJasonFile)
}

func (iClient *Client) getBookInfoInjectHTTPAbortToRatingsForJasonFile() (string, error) {
	return iClient.getCpxComponentYAML(bookInfoInjectHTTPAbortToRatingsForJasonFile)
}
