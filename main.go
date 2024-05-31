package main

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"github.com/pkg/sftp"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/crypto/ssh"
)

var (
	wg          sync.WaitGroup
	jobs        chan string = make(chan string)
	downloadDir string      = "./download"
)

func init() {
	err := godotenv.Load(".env")
	if err != nil {
		os.Setenv("IP_ADDR", "")
		os.Setenv("SFTP_USER", "")
		os.Setenv("SFTP_PASS", "")
	}
}

func main() {
	var conn *ssh.Client = createConnection()
	client, err := sftp.NewClient(conn)
	if err != nil {
		fmt.Println(err)
	}

	session, err := conn.NewSession()
	if err != nil {
		fmt.Println(err)
	}

	defer conn.Close()
	defer client.Close()
	defer session.Close()

	fmt.Println("Established connection to sftp server.")

	var maxCount int
	rootDir := ".kodi"
	output, err := session.Output("find .kodi -type f | wc -l")
	fmt.Sscanf(string(output), "%d\n", &maxCount)

	os.MkdirAll(downloadDir, 0777)

	/* dirBar := progressbar.Default() */
	fmt.Println("Creating dirs. Please wait...")
	loopDirs(rootDir, client)

	if err != nil {
		fmt.Println(err)
	}

	filesBar := progressbar.Default(int64(maxCount))
	for w := 1; w <= 20; w++ {
		go loopFiles(client, filesBar)
	}

	wg.Wait()
}

func downloadFile(client *sftp.Client, remoteFile string, destFile string) error {
	localPath := downloadDir + destFile[5:]

	srcFile, err := client.OpenFile(remoteFile, (os.O_RDONLY))
	if err != nil {
		return err
	}

	defer srcFile.Close()

	dstFile, err := os.Create(localPath)
	if err != nil {
		return err
	}

	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return nil
}

func loopFiles(client *sftp.Client, bar *progressbar.ProgressBar) error {
	for path := range jobs {
		files, err := client.ReadDir(path)
		if err != nil {
			wg.Done()
			fmt.Println(err)

			return err
		}

		for _, file := range files {
			if !file.IsDir() {
				destPath := path + "/" + file.Name()
				downloadFile(client, path+"/"+file.Name(), destPath)

				bar.Add(1)
			}
		}

		wg.Done()
	}

	return nil
}

func loopDirs(path string, client *sftp.Client) error {
	files, err := client.ReadDir(path)
	if err != nil {
		return err
	}

	go func() {
		wg.Add(1)
		jobs <- path
	}()

	for _, file := range files {
		if file.IsDir() {
			_, err := os.Stat(downloadDir + path[5:] + "/" + file.Name())

			if os.IsNotExist(err) {
				err := os.MkdirAll(downloadDir+path[5:]+"/"+file.Name(), 0777)

				if err != nil {
					fmt.Println(err)
				}

				os.Create(downloadDir + path[5:] + "/" + file.Name())
			}

			loopDirs(path+"/"+file.Name(), client)
		}
	}

	return nil
}

func createConnection() *ssh.Client {
	config := &ssh.ClientConfig{
		User: os.Getenv("SFTP_USER"),
		Auth: []ssh.AuthMethod{
			ssh.Password(os.Getenv("SFTP_PASS")),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", os.Getenv("IP_ADDR")+":22", config)
	if err != nil {
		fmt.Println(err)
	}

	return client
}
