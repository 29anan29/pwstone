package model

import "errors"

type Entry struct {
	Site     string `json:"site"`
	Username string `json:"username"`
	Password string `json:"password"`
	Notes    string `json:"notes,omitempty"`
}

func (e Entry) ID() string { return e.Site + "\x00" + e.Username }

func (e Entry) Validate() error {
	if e.Site == "" {
		return errors.New("网站不能为空")
	}
	if e.Username == "" {
		return errors.New("账号不能为空")
	}
	if e.Password == "" {
		return errors.New("密码不能为空")
	}
	return nil
}
