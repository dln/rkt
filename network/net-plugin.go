package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/coreos/rocket/network/util"
)

type NetPlugin struct {
	Name     string `json:"name,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Command  struct {
		Add []string `json:"add,omitempty"`
		Del []string `json:"del,omitempty"`
	}
}

const RktNetPluginsPath = "/etc/rkt-net-plugins.conf.d"

func LoadNetPlugin(path string) (*NetPlugin, error) {
	c, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	np := &NetPlugin{}
	if err = json.Unmarshal(c, np); err != nil {
		return nil, err
	}

	return np, nil
}

func LoadNetPlugins() (map[string]*NetPlugin, error) {
	plugins := make(map[string]*NetPlugin)

	dirents, err := ioutil.ReadDir(RktNetPluginsPath)
	switch {
	case err == nil:
	case os.IsNotExist(err):
		return plugins, nil
	default:
		return nil, err
	}

	for _, dent := range dirents {
		if dent.IsDir() {
			continue
		}

		npPath := filepath.Join(RktNetPluginsPath, dent.Name())
		np, err := LoadNetPlugin(npPath)
		if err != nil {
			log.Printf("Loading %v: %v", npPath, err)
			continue
		}

		plugins[np.Name] = np
	}

	return plugins, nil
}

func (np *NetPlugin) Add(n *Net, contID, netns, args, ifName string) (*net.IPNet, error) {
	switch {
	case np.Endpoint != "":
		return nil, execHTTP(np.Endpoint, "add", n.Name, contID, netns, n.Filename, args, ifName)

	default:
		if len(np.Command.Add) == 0 {
			return nil, fmt.Errorf("plugin does not define command.add")
		}

		output, err := execCmd(np.Command.Add, n.Name, contID, netns, n.Filename, args, ifName)
		if err != nil {
			return nil, err
		}

		fmt.Printf("plugin's output %q\n", output)

		return util.ParseCIDR(output)
	}
}

func (np *NetPlugin) Del(n *Net, contID, netns, args, ifName string) error {
	switch {
	case np.Endpoint != "":
		return execHTTP(np.Endpoint, "del", n.Name, contID, netns, n.Filename, args, ifName)

	default:
		if len(np.Command.Del) == 0 {
			return fmt.Errorf("plugin does not define command.del")
		}

		_, err := execCmd(np.Command.Del, n.Name, contID, netns, n.Filename, args, ifName)
		return err
	}
}

func execHTTP(ep, cmd, netName, contID, netns, confFile, args, ifName string) error {
	return fmt.Errorf("not implemented")
}

func replaceAll(xs []string, what, with string) {
	for i, x := range xs {
		xs[i] = strings.Replace(x, what, with, -1)
	}
}

func execCmd(cmd []string, netName, contID, netns, confFile, args, ifName string) (string, error) {
	replaceAll(cmd, "{net-name}", netName)
	replaceAll(cmd, "{cont-id}", contID)
	replaceAll(cmd, "{netns}", netns)
	replaceAll(cmd, "{conf-file}", confFile)
	replaceAll(cmd, "{args}", args)
	replaceAll(cmd, "{if-name}", ifName)

	stdout := &bytes.Buffer{}

	c := exec.Command(cmd[0], cmd[1:]...)
	c.Stdout = stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return "", err
	}

	return stdout.String(), nil
}
