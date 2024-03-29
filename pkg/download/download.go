package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/firmwareci/contest-client/pkg/env"
	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// DownloadBinary parses the unparsed URL, and proceeds to download
func DownloadBinary(unparsedURL string) (string, error) {
	URL, err := url.Parse(unparsedURL)
	if err != nil {
		return "", err
	}
	filename := path.Base(URL.Path)
	fmt.Printf("file: %v   URL: %v", filename, URL)

	binaryDirectory := os.Getenv(env.EnvBinDir)

	if binaryDirectory != "" {
		if path.IsAbs(binaryDirectory) != true {
			return "", fmt.Errorf("Path: '%s', is not absolute", binaryDirectory)
		}
	} else {
		binaryDirectory = env.DefaultBinaryDir
	}

	dir, filepath := filepath.Split(binaryDirectory)
	if filepath != "" {
		return "", fmt.Errorf("supplied download directory is not a valid directory path, it seems like a /path/to/file")
	}

	_, err = os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				return "", fmt.Errorf("path to download the binary does not exist, error while creating it: %v", err)
			}
		} else {
			return "", fmt.Errorf("error retrieving binary path stats: %v", err)
		}
	}

	binaryPath := path.Join(binaryDirectory, filename)
	fmt.Printf("Binary Path: %v", binaryPath)

	file, err := os.Create(binaryPath)
	if err != nil {
		return "", err
	}

	defer file.Close()

	switch URL.Scheme {
	case "http", "https", "":
		if err := downloadHTTP(file, URL); err != nil {
			return "", err
		}
	case "ftp":
		if err := downloadFTP(file, URL); err != nil {
			return "", err
		}
	case "sftp":
		if err := downloadSFTP(file, URL); err != nil {
			return "", err
		}
	case "s3":
		if err := downloadS3(file, URL); err != nil {
			return "", err
		}

	}
	return binaryPath, nil
}

// DownloadHttp downloads the file at the end of the Url via http/ https,
// into the given file. Both pointers owned by caller.
func downloadHTTP(file *os.File, URL *url.URL) error {
	resp, err := http.Get(URL.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return err
	}
	return nil
}

// DownloadFtp downloads the file at the end of the Url via ftp,
// into the given file. Both pointers owned by caller.
func downloadFTP(file *os.File, URL *url.URL) error {
	if URL.User == nil {
		URL.User = url.UserPassword("anonymous", "anonymous")
	}

	pw, set := URL.User.Password()

	if !set {
		pw = "anonymous"
	}

	connection, err := ftp.DialTimeout(URL.Hostname(), 60*time.Second)
	if err != nil {
		return err
	}

	defer connection.Quit()

	if err := connection.Login(URL.User.Username(), pw); err != nil {
		return err
	}

	resp, err := connection.Retr(URL.Path)
	if err != nil {
		return err
	}

	defer resp.Close()

	if _, err := io.Copy(file, resp); err != nil {
		return err
	}

	return nil
}

// DownloadSFTP downloads the file at the end of the Url via sftp,
// into the given file. Both pointers owned by caller.
func downloadSFTP(file *os.File, URL *url.URL) error {
	if URL.User == nil {
		return errors.New("No User Account or password provided")
	}

	keyFile, err := os.Open(env.FirmwareciSSHKey)
	if err != nil {
		return err
	}

	sshKey, err := io.ReadAll(keyFile)
	if err != nil {
		return err
	}

	var Signer ssh.Signer

	if sshPW, set := os.LookupEnv(env.EnvSSHPrivatePW); set == false || sshPW != "" {
		Signer, err = ssh.ParsePrivateKeyWithPassphrase(sshKey, []byte(sshPW))
	} else {
		Signer, err = ssh.ParsePrivateKey(sshKey)
	}

	if err != nil {
		return err
	}

	AuthMethods := []ssh.AuthMethod{ssh.PublicKeys(Signer)}

	var hostPublicKey ssh.PublicKey

	if key, set := os.LookupEnv(env.EnvSSHHostPublic); set == false || key != "" {
		hostPublicKey, err = ssh.ParsePublicKey([]byte(key))
		if err != nil {
			return err
		}
	}

	sshClient, err := ssh.Dial("tcp", URL.Host, &ssh.ClientConfig{
		User:            URL.User.Username(),
		Auth:            AuthMethods,
		HostKeyCallback: ssh.FixedHostKey(hostPublicKey),
	})
	if err != nil {
		return err
	}
	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return err
	}

	defer sftpClient.Close()

	binFile, err := sftpClient.Open(URL.Path)
	if err != nil {
		return err
	}

	defer binFile.Close()

	if _, err := io.Copy(file, binFile); err != nil {
		return err
	}

	return nil
}

// DownloadS3 downloads the file from a S3 bucket, using the aws go api.
// Both pointers owned by caller.
func downloadS3(file *os.File, URL *url.URL) error {
	access, set := os.LookupEnv(env.EnvS3Access)
	if set == false || access == "" {
		return fmt.Errorf("No Access token specified for the S3 bucket")
	}

	secret, set := os.LookupEnv(env.EnvS3Secret)
	if set == false || secret == "" {
		return fmt.Errorf("No Secret token specified for the S3 bucket")
	}

	region, set := os.LookupEnv(env.EnvS3Region)
	if set == false || region == "" {
		return fmt.Errorf("No region specified for the S3 bucket")
	}

	creds := credentials.NewStaticCredentialsProvider(access, secret, "")

	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithCredentialsProvider(creds), config.WithRegion(region))
	if err != nil {
		return err
	}

	S3client := s3.NewFromConfig(cfg)

	Bucket := &URL.Host
	Key := strings.TrimPrefix(URL.Path, "/")

	out, err := S3client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: Bucket,
		Key:    &Key,
	})
	if err != nil {
		return err
	}

	defer out.Body.Close()

	if _, err := io.Copy(file, out.Body); err != nil {
		return err
	}

	return nil
}
