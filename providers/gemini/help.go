package gemini

// Examples is the block rendered under EXAMPLES in `imagine --help` when
// Gemini (or Vertex, which reuses it) is the active provider. The
// command layer auto-prepends the ACTIVE PROVIDER line and the MODELS
// list from Info, so this only returns the bespoke examples + sizes.
func Examples() string {
	return `  imagine -p "a sunset" -n 3 -s 2K -a 16:9
  imagine -p "futuristic city" -m flash -t high
  imagine -p "sticker of an avocado" -m flash-lite -a 1:1
  imagine -p "cat in hoodie" -I
  imagine -p "thumbnail" -m flash -s 512
  imagine -p "add rain" -i photo.png -r

  SIZES:
    512, 1K, 2K, 4K  (default: 1K; 512 is flash only, flash-lite is 1K only)`
}
