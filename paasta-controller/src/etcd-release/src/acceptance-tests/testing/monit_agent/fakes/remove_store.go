package fakes

type RemoveStore struct {
	DeleteContentsCall struct {
		CallCount int
		Receives  struct {
			StoreDir string
		}
		Returns struct {
			Error error
		}
	}
}

func (r *RemoveStore) DeleteContents(storeDir string) error {
	r.DeleteContentsCall.CallCount++
	r.DeleteContentsCall.Receives.StoreDir = storeDir
	return r.DeleteContentsCall.Returns.Error
}
