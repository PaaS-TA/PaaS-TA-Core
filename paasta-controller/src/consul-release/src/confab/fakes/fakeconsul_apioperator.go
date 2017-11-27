package fakes

import "github.com/hashicorp/consul/api"

type FakeconsulAPIOperator struct {
	KeyringListCall struct {
		CallCount int
		Receives  struct {
			QueryOptions *api.QueryOptions
		}
		Returns struct {
			KeyringResponse []*api.KeyringResponse
			Error           error
		}
	}

	KeyringInstallCall struct {
		CallCount int
		Receives  struct {
			Key          string
			WriteOptions *api.WriteOptions
		}
		Returns struct {
			Error error
		}
	}

	KeyringUseCall struct {
		CallCount int
		Receives  struct {
			Key          string
			WriteOptions *api.WriteOptions
		}
		Returns struct {
			Error error
		}
	}

	KeyringRemoveCall struct {
		CallCount int
		Receives  struct {
			Key          string
			WriteOptions *api.WriteOptions
		}
		Returns struct {
			Error error
		}
	}
}

func (o *FakeconsulAPIOperator) KeyringList(queryOptions *api.QueryOptions) ([]*api.KeyringResponse, error) {
	o.KeyringListCall.CallCount++
	o.KeyringListCall.Receives.QueryOptions = queryOptions
	return o.KeyringListCall.Returns.KeyringResponse, o.KeyringListCall.Returns.Error
}

func (o *FakeconsulAPIOperator) KeyringInstall(key string, writeOptions *api.WriteOptions) error {
	o.KeyringInstallCall.CallCount++
	o.KeyringInstallCall.Receives.Key = key
	o.KeyringInstallCall.Receives.WriteOptions = writeOptions
	return o.KeyringInstallCall.Returns.Error
}

func (o *FakeconsulAPIOperator) KeyringUse(key string, writeOptions *api.WriteOptions) error {
	o.KeyringUseCall.CallCount++
	o.KeyringUseCall.Receives.Key = key
	o.KeyringUseCall.Receives.WriteOptions = writeOptions
	return o.KeyringUseCall.Returns.Error
}

func (o *FakeconsulAPIOperator) KeyringRemove(key string, writeOptions *api.WriteOptions) error {
	o.KeyringRemoveCall.CallCount++
	o.KeyringRemoveCall.Receives.Key = key
	o.KeyringRemoveCall.Receives.WriteOptions = writeOptions
	return o.KeyringRemoveCall.Returns.Error
}
