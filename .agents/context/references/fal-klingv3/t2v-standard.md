# Kling Video v3 Text to Video [Standard]

> Kling 3.0 Standard: Top-tier text-to-video with cinematic visuals, fluid motion, and native audio generation, with multi-shot support.


## Overview

- **Endpoint**: `https://fal.run/fal-ai/kling-video/v3/standard/text-to-video`
- **Model ID**: `fal-ai/kling-video/v3/standard/text-to-video`
- **Category**: text-to-video
- **Kind**: inference
**Tags**: text-to-video



## Pricing

For every second of video you generated, you will be charged **$0.084** (audio off) or **$0.126** (audio on), if voice control is used while generating audio you will be charged **$0.154**. For example, a 5s video with audio on and voice control will cost **$0.77**

For more details, see [fal.ai pricing](https://fal.ai/pricing).

## API Information

This model can be used via our HTTP API or more conveniently via our client libraries.
See the input and output schema below, as well as the usage examples.


### Input Schema

The API accepts the following input parameters:


- **`prompt`** (`string`, _optional_):
  Text prompt for video generation. Either prompt or multi_prompt must be provided, but not both.
  - Examples: "Cinematic drone shot flying through ancient stone ruins covered in moss and vines at golden hour. Camera starts low, rises through crumbling archways, revealing a vast misty valley beyond. Volumetric light rays pierce through gaps in the stone. Epic scale, photorealistic, 8K quality."

- **`duration`** (`DurationEnum`, _optional_):
  The duration of the generated video in seconds Default value: `"5"`
  - Default: `"5"`
  - Options: `"3"`, `"4"`, `"5"`, `"6"`, `"7"`, `"8"`, `"9"`, `"10"`, `"11"`, `"12"`, `"13"`, `"14"`, `"15"`

- **`multi_prompt`** (`list<KlingV3MultiPromptElement>`, _optional_):
  List of prompts for multi-shot video generation. If provided, overrides the single prompt and divides the video into multiple shots with specified prompts and durations.
  - Array of KlingV3MultiPromptElement
  - Examples: null

- **`generate_audio`** (`boolean`, _optional_):
  Whether to generate native audio for the video. Supports Chinese and English voice output. Other languages are automatically translated to English. For English speech, use lowercase letters; for acronyms or proper nouns, use uppercase. Default value: `true`
  - Default: `true`

- **`shot_type`** (`ShotTypeEnum`, _optional_):
  The type of multi-shot video generation. 'intelligent' lets the model automatically determine shot structure. Default value: `"customize"`
  - Default: `"customize"`
  - Options: `"customize"`, `"intelligent"`

- **`aspect_ratio`** (`AspectRatioEnum`, _optional_):
  The aspect ratio of the generated video frame Default value: `"16:9"`
  - Default: `"16:9"`
  - Options: `"16:9"`, `"9:16"`, `"1:1"`

- **`negative_prompt`** (`string`, _optional_):
   Default value: `"blur, distort, and low quality"`
  - Default: `"blur, distort, and low quality"`

- **`cfg_scale`** (`float`, _optional_):
  The CFG (Classifier Free Guidance) scale is a measure of how close you want
  the model to stick to your prompt. Default value: `0.5`
  - Default: `0.5`
  - Range: `0` to `1`



**Required Parameters Example**:

```json
{}
```

**Full Example**:

```json
{
  "prompt": "Cinematic drone shot flying through ancient stone ruins covered in moss and vines at golden hour. Camera starts low, rises through crumbling archways, revealing a vast misty valley beyond. Volumetric light rays pierce through gaps in the stone. Epic scale, photorealistic, 8K quality.",
  "duration": "5",
  "multi_prompt": null,
  "generate_audio": true,
  "shot_type": "customize",
  "aspect_ratio": "16:9",
  "negative_prompt": "blur, distort, and low quality",
  "cfg_scale": 0.5
}
```


### Output Schema

The API returns the following output format:

- **`video`** (`File`, _required_):
  The generated video
  - Examples: {"file_size":6797486,"content_type":"video/mp4","file_name":"output.mp4","url":"https://storage.googleapis.com/falserverless/example_outputs/kling-v3/standard-t2v/out.mp4"}



**Example Response**:

```json
{
  "video": {
    "file_size": 6797486,
    "content_type": "video/mp4",
    "file_name": "output.mp4",
    "url": "https://storage.googleapis.com/falserverless/example_outputs/kling-v3/standard-t2v/out.mp4"
  }
}
```


## Usage Examples

### cURL

```bash
curl --request POST \
  --url https://fal.run/fal-ai/kling-video/v3/standard/text-to-video \
  --header "Authorization: Key $FAL_KEY" \
  --header "Content-Type: application/json" \
  --data '{}'
```

### Python

Ensure you have the Python client installed:

```bash
pip install fal-client
```

Then use the API client to make requests:

```python
import fal_client

def on_queue_update(update):
    if isinstance(update, fal_client.InProgress):
        for log in update.logs:
           print(log["message"])

result = fal_client.subscribe(
    "fal-ai/kling-video/v3/standard/text-to-video",
    arguments={},
    with_logs=True,
    on_queue_update=on_queue_update,
)
print(result)
```

### JavaScript

Ensure you have the JavaScript client installed:

```bash
npm install --save @fal-ai/client
```

Then use the API client to make requests:

```javascript
import { fal } from "@fal-ai/client";

const result = await fal.subscribe("fal-ai/kling-video/v3/standard/text-to-video", {
  input: {},
  logs: true,
  onQueueUpdate: (update) => {
    if (update.status === "IN_PROGRESS") {
      update.logs.map((log) => log.message).forEach(console.log);
    }
  },
});
console.log(result.data);
console.log(result.requestId);
```


## Additional Resources

### Documentation

- [Model Playground](https://fal.ai/models/fal-ai/kling-video/v3/standard/text-to-video)
- [API Documentation](https://fal.ai/models/fal-ai/kling-video/v3/standard/text-to-video/api)
- [OpenAPI Schema](https://fal.ai/api/openapi/queue/openapi.json?endpoint_id=fal-ai/kling-video/v3/standard/text-to-video)

### fal.ai Platform

- [Platform Documentation](https://docs.fal.ai)
- [Python Client](https://docs.fal.ai/clients/python)
- [JavaScript Client](https://docs.fal.ai/clients/javascript)
