package imagenes

import (
	"context"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
)

// AzureBlob guarda objetos en un contenedor de Azure Blob Storage (ADR-0013).
// En local se prueba contra Azurite con su cadena de conexión.
type AzureBlob struct {
	c         *azblob.Client
	container string
}

// NewAzureBlob crea el cliente desde una cadena de conexión y asegura que el contenedor exista (privado).
func NewAzureBlob(ctx context.Context, connString, container string) (*AzureBlob, error) {
	c, err := azblob.NewClientFromConnectionString(connString, nil)
	if err != nil {
		return nil, err
	}
	if _, err := c.CreateContainer(ctx, container, nil); err != nil && !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
		return nil, err
	}
	return &AzureBlob{c: c, container: container}, nil
}

func (a *AzureBlob) Put(ctx context.Context, key string, data []byte, ct string) error {
	_, err := a.c.UploadBuffer(ctx, a.container, key, data, &azblob.UploadBufferOptions{
		HTTPHeaders: &blob.HTTPHeaders{BlobContentType: to.Ptr(ct), BlobCacheControl: to.Ptr("public, max-age=31536000, immutable")},
	})
	return err
}

func (a *AzureBlob) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	r, err := a.c.DownloadStream(ctx, a.container, key, nil)
	if bloberror.HasCode(err, bloberror.BlobNotFound) {
		return nil, ErrNoExiste
	}
	if err != nil {
		return nil, err
	}
	return r.Body, nil
}

func (a *AzureBlob) Exists(ctx context.Context, key string) (bool, error) {
	_, err := a.c.ServiceClient().NewContainerClient(a.container).NewBlobClient(key).GetProperties(ctx, nil)
	if bloberror.HasCode(err, bloberror.BlobNotFound) {
		return false, nil
	}
	return err == nil, err
}
