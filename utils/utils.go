package utils

import (
	"github.com/coreos/etcd/client"
)

func CheckExistsError(err error) error {
	//FIXME indicate path exists
	if etcdError, ok := err.(client.Error); ok {
		if etcdError.Code == client.ErrorCodeNodeExist {
			return nil
		}
	}
	return err
}
