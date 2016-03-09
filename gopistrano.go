package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/alanchavez88/goconf"
	"golang.org/x/crypto/ssh"
)

var (
	user,
	pass,
	hostname,
	port,
	repository,
	path,
	releases,
	shared,
	utils,
	privateKey,
	keepReleases string
)

type deploy struct {
	cl *ssh.Client
}

func init() {
	var err error
	c, err := conf.ReadConfigFile("Gopfile")

	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	user, err = c.GetString("", "username")
	pass, err = c.GetString("", "password")
	privateKey, err = c.GetString("", "private_key")
	hostname, err = c.GetString("", "hostname")
	repository, err = c.GetString("", "repository")
	port, err = c.GetString("", "port")
	path, err = c.GetString("", "path")
	releases = path + "/releases"
	shared = path + "/shared"
	utils = path + "/utils"

	keepReleases, err = c.GetString("", "keep_releases")

	// just log whichever we get; let the user re-run the program to see all errors..
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func main() {
	flag.Parse()

	action := flag.Arg(0)

	if action == "" {
		fmt.Println("Error: use gopistrano deploy or gopistrano deploy:setup")
		return
	}

	deploy, err := newDeploy()

	// Do panic if the dial fails
	if err != nil {
		fmt.Println("Failed to start: " + err.Error())
		return
	}

	switch strings.ToLower(action) {
	case "deploy:setup":
		err = deploy.Setup()
	case "deploy":
		err = deploy.Run()
	default:
		fmt.Println("Invalid command!")
	}

	if err != nil {
		fmt.Println(err.Error())
	}
}

// returns a new deployment
func newDeploy() (d *deploy, err error) {
	if pass != "" && user != "" {
		cfg := &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				ssh.Password(pass),
			},
		}

		fmt.Println("SSH-ing into " + hostname)
		cl, err := ssh.Dial("tcp", hostname+":"+port, cfg)

		if err != nil {
			return nil, err
		}

		d = &deploy{cl: cl}
	} else if privateKey != "" && user != "" {
		cfg := &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				publicKeyFile(privateKey),
			},
		}

		fmt.Println("SSH-ing into " + hostname + " with private key " + privateKey)
		cl, err := ssh.Dial("tcp", hostname+":"+port, cfg)

		if err != nil {
			return nil, err
		}

		d = &deploy{cl: cl}
	} else {
		panic("Unable to authenticate with server")
	}
	return
}

func publicKeyFile(file string) ssh.AuthMethod {
	buffer, err := ioutil.ReadFile(file)

	if err != nil {
		return nil
	}

	key, err := ssh.ParsePrivateKey(buffer)

	if err != nil {
		return nil
	}

	return ssh.PublicKeys(key)
}

// runs the deployment script remotely
func (d *deploy) Run() error {
	deployCmd := "if [ ! -d " + releases + " ]; then exit 1; fi &&" +
		"if [ ! -d " + shared + " ]; then exit 1; fi &&" +
		"if [ ! -d " + utils + " ]; then exit 1; fi &&" +
		"if [ ! -f " + utils + "/deploy.sh ]; then exit 1; fi &&" +
		"" + utils + "/deploy.sh " + path + " " + repository + " " + keepReleases

	if err := d.runCmd(deployCmd); err != nil {
		return err
	}

	fmt.Println("Project Deployed!")
	return nil
}

// sets up directories for deployment a la capistrano
func (d *deploy) Setup() error {
	cdPathCmd := "if [ ! -d " + releases + " ]; then mkdir " + releases + "; fi &&" +
		"if [ ! -d " + shared + " ]; then mkdir " + shared + "; fi &&" +
		"if [ ! -d " + utils + " ]; then mkdir " + utils + "; fi &&" +
		"chmod g+w " + releases + " " + shared + " " + path + " " + utils

	if err := d.runCmd(cdPathCmd); err != nil {
		return err
	}

	fmt.Println("Running scp connection")

	cpy := `echo -n '` + string(deployment_script) + `' > ` + utils + `/deploy.sh ; chmod +x ` + utils + `/deploy.sh`

	if err := d.runCmd(cpy); err != nil {
		return err
	}

	fmt.Println("Cool Beans! Gopistrano created the structure correctly!")
	return nil
}

// basic ssh cmd runner
func (d *deploy) runCmd(cmd string) (err error) {
	session, err := d.cl.NewSession()
	if err != nil {
		return err
	}

	//this *does* return an error (EOF of some sort), but I guess we don't care?
	//the ssh lib needs to send it and must return it or something
	defer session.Close()

	//send through to main stdout, stderr
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	return session.Run(cmd)
}
