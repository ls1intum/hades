package docker

import (
	"context"
	"fmt"
	"io"
	"sync"

	imagetype "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

func pullImages(ctx context.Context, client *client.Client, images ...string) error {
	var wg sync.WaitGroup
	errorsCh := make(chan error, len(images))

	for _, image := range images {
		wg.Add(1)

		go func(img string) {
			defer wg.Done()

			response, err := client.ImagePull(ctx, img, imagetype.PullOptions{})
			if err != nil {
				errorsCh <- fmt.Errorf("failed to pull image %s: %v", img, err)
				return
			}
			defer response.Close()
			io.Copy(io.Discard, response) // consume the response to prevent potential leaks
		}(image)
	}

	// wait for all goroutines to complete
	wg.Wait()
	close(errorsCh)

	// Collect errors
	var errors []error
	for err := range errorsCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors while pulling images: %+v", len(errors), errors)
	}

	return nil
}
