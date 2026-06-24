package fal

// Examples is the block rendered under EXAMPLES in `imagine --help` when fal
// is the active provider. The command layer auto-prepends the ACTIVE PROVIDER
// line and the MODELS list from Info, so this only returns the bespoke
// examples + sizes.
func Examples() string {
	return `  imagine providers add fal        # store your fal.ai API key (FAL_KEY)
  imagine -p "a fox leaping through tall grass"            # text → video
  imagine -p "slow zoom on the city skyline" -s 1080p -n 2
  imagine -p "make it morning" -i frame.png               # image → video
  imagine -p "pan to the right" -i start.png --end-image end.png
  imagine -p "match this look" -i ref1.png -i ref2.png    # reference → video

  RESOLUTIONS:
    480p   720p (default)   1080p

  ASPECT RATIOS:
    auto (default)   21:9   16:9   4:3   1:1   3:4   9:16`
}
