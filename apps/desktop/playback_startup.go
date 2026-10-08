package main

import "context"

func startPlayback(ctx context.Context, cancel context.CancelFunc, play func(context.Context) error, loadEpisodes func(context.Context)) error {
	if err := play(ctx); err != nil {
		cancel()
		return err
	}
	go func() {
		defer cancel()
		if ctx.Err() == nil {
			loadEpisodes(ctx)
		}
	}()
	return nil
}
