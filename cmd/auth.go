// Copyright 2015 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"golang.org/x/crypto/ssh/terminal"
)

type loginScheme struct {
	Name string
	Data map[string]string
}

type login struct {
	scheme *loginScheme
}

func nativeLogin(context *Context, client *Client) error {
	var email string
	if len(context.Args) > 0 {
		email = context.Args[0]
	} else {
		fmt.Fprint(context.Stdout, "Email: ")
		fmt.Fscanf(context.Stdin, "%s\n", &email)
	}
	fmt.Fprint(context.Stdout, "Password: ")
	password, err := passwordFromReader(context.Stdin)
	if err != nil {
		return err
	}
	fmt.Fprintln(context.Stdout)
	url, err := GetURL("/users/" + email + "/tokens")
	if err != nil {
		return err
	}
	b := bytes.NewBufferString(`{"password":"` + password + `"}`)
	request, err := http.NewRequest("POST", url, b)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	result, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	out := make(map[string]interface{})
	err = json.Unmarshal(result, &out)
	if err != nil {
		return err
	}
	fmt.Fprintln(context.Stdout, "Successfully logged in!")
	return writeToken(out["token"].(string))
}

func (c *login) getScheme() *loginScheme {
	if c.scheme == nil {
		info, err := schemeInfo()
		if err != nil {
			c.scheme = &loginScheme{Name: "native", Data: make(map[string]string)}
		} else {
			c.scheme = info
		}
	}
	return c.scheme
}

func (c *login) Run(context *Context, client *Client) error {
	if c.getScheme().Name == "oauth" {
		return c.oauthLogin(context, client)
	}
	return nativeLogin(context, client)
}

func (c *login) Info() *Info {
	usage := "login [email]"
	return &Info{
		Name:  "login",
		Usage: usage,
		Desc: `Initiates a new tsuru session for a user. If using tsuru native authentication
scheme, it will ask for the email and the password and check if the user is
successfully authenticated. If using OAuth, it will open a web browser for the
user to complete the login.

After that, the token generated by the tsuru server will be stored in
[[${HOME}/.tsuru_token]].

All tsuru actions require the user to be authenticated (except [[tsuru login]]
and [[tsuru version]]).`,
		MinArgs: 0,
	}
}

type logout struct{}

func (c *logout) Info() *Info {
	return &Info{
		Name:  "logout",
		Usage: "logout",
		Desc: `Logout will delete the token file and terminate the session within tsuru
server.`,
	}
}

func (c *logout) Run(context *Context, client *Client) error {
	if url, err := GetURL("/users/tokens"); err == nil {
		request, _ := http.NewRequest("DELETE", url, nil)
		client.Do(request)
	}
	err := filesystem().Remove(JoinWithUserDir(".tsuru_token"))
	if err != nil && os.IsNotExist(err) {
		return errors.New("You're not logged in!")
	}
	fmt.Fprintln(context.Stdout, "Successfully logged out!")
	return nil
}

func passwordFromReader(reader io.Reader) (string, error) {
	var (
		password []byte
		err      error
	)
	if desc, ok := reader.(descriptable); ok && terminal.IsTerminal(int(desc.Fd())) {
		password, err = terminal.ReadPassword(int(desc.Fd()))
		if err != nil {
			return "", err
		}
	} else {
		fmt.Fscanf(reader, "%s\n", &password)
	}
	if len(password) == 0 {
		msg := "You must provide the password!"
		return "", errors.New(msg)
	}
	return string(password), err
}

func schemeInfo() (*loginScheme, error) {
	url, err := GetURL("/auth/scheme")
	if err != nil {
		return nil, err
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	info := loginScheme{}
	err = json.NewDecoder(resp.Body).Decode(&info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}
