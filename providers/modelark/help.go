package modelark

// Examples is the block rendered under EXAMPLES in `imagine --help` when
// modelark is the active provider. The command layer auto-prepends the ACTIVE
// PROVIDER line and the MODELS list from Info.
func Examples() string {
	return `  imagine storage set              # configure an S3-compatible bucket (required)
  imagine providers add modelark   # store your ModelArk API key (ARK_API_KEY)
  imagine -p "a fox leaping through tall grass"            # text → video
  imagine -p "slow zoom on the skyline" -s 1080p           # full model, 1080p
  imagine -p "make it morning" -i frame.png                # image → video
  imagine -p "pan to the right" -i start.png --end-image end.png   # first+last frame
  imagine -p "match this look" -i ref1.png -i clip.mp4     # reference → video

  RESOLUTIONS:
    480p   720p (default)   1080p (full only)   4k (full only)
    Fast and Mini cap at 720p. 4k is H.265/10-bit — some players/browsers
    may not play it directly.

  ASPECT RATIOS:
    adaptive (default)   21:9   16:9   4:3   1:1   3:4   9:16

  NOTE: references are uploaded to a dedicated, public-read S3-compatible
  bucket (BytePlus TOS, MinIO, Cloudflare R2, …) and fetched server-side, so
  the bucket must allow anonymous reads. Seedance rejects references
  containing real human faces.`
}
