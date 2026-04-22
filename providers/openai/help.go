package openai

// Examples is the block rendered under EXAMPLES in `imagine --help` when
// OpenAI is the active provider. The command layer auto-prepends the
// ACTIVE PROVIDER line and the MODELS list from Info, so this only
// returns the bespoke examples + sizes.
func Examples() string {
	return `  imagine -p "a red apple" -q low
  imagine -p "logo" -n 3 -s 1024x1024
  imagine -p "hero banner" -s 3840x2160 -q high -f hero.jpg
  imagine -p "sticker" -m 1.5 --background transparent -f sticker.png
  imagine -p "make it winter" -i photo.png

  SIZES — shorthand:
    1K → 1024x1024
    2K → 2048x2048
    4K → 3840x2160
    auto  (model picks — default)

  SIZES — popular raw dimensions:
    1024x1024  (square)
    1536x1024  (landscape)
    1024x1536  (portrait)
    2048x2048  (2K square)
    2048x1152  (2K landscape)
    3840x2160  (4K landscape)
    2160x3840  (4K portrait)

  Any WxH is accepted if: edge ≤ 3840px, both multiples of 16, ratio ≤ 3:1, pixels 655,360–8,294,400

  Edit mode (-i set): only 1024x1024, 1536x1024, 1024x1536, auto`
}
