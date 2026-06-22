# Kling Video V3 Text to Video 4K

> Kling's Native 4K is a video generation model that directly outputs professional-grade 4K video in one step, eliminating the need for post-production upscaling


## Overview

- **Endpoint**: `https://fal.run/fal-ai/kling-video/v3/4k/text-to-video`
- **Model ID**: `fal-ai/kling-video/v3/4k/text-to-video`
- **Category**: text-to-video
- **Kind**: inference
**Tags**: stylized, transform, lipsync



## Pricing

For every second of video you generated, you will be charged **$0.42** regardless of whether audio is on or off. For example, a **5s** video will cost **$2.10**.

For more details, see [fal.ai pricing](https://fal.ai/pricing).

## API Information

This model can be used via our HTTP API or more conveniently via our client libraries.
See the input and output schema below, as well as the usage examples.


### Input Schema

The API accepts the following input parameters:


- **`prompt`** (`string`, _optional_):
  Text prompt for video generation. Either prompt or multi_prompt must be provided, but not both.
  - Examples: "Close-up of glowing fireflies dancing in a dark forest at twilight. Soft bioluminescent particles float through the air. Shallow depth of field, bokeh lights in background. Magical atmosphere, gentle movement."

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
  "prompt": "Close-up of glowing fireflies dancing in a dark forest at twilight. Soft bioluminescent particles float through the air. Shallow depth of field, bokeh lights in background. Magical atmosphere, gentle movement.",
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
  - Examples: {"file_size":8062911,"content_type":"video/mp4","file_name":"output.mp4","url":"https://storage.googleapis.com/falserverless/example_outputs/kling-v3/pro-t2v/out.mp4"}



**Example Response**:

```json
{
  "video": {
    "file_size": 8062911,
    "content_type": "video/mp4",
    "file_name": "output.mp4",
    "url": "https://storage.googleapis.com/falserverless/example_outputs/kling-v3/pro-t2v/out.mp4"
  }
}
```


## Usage Examples

### cURL

```bash
curl --request POST \
  --url https://fal.run/fal-ai/kling-video/v3/4k/text-to-video \
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
    "fal-ai/kling-video/v3/4k/text-to-video",
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

const result = await fal.subscribe("fal-ai/kling-video/v3/4k/text-to-video", {
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

- [Model Playground](https://fal.ai/models/fal-ai/kling-video/v3/4k/text-to-video)
- [API Documentation](https://fal.ai/models/fal-ai/kling-video/v3/4k/text-to-video/api)
- [OpenAPI Schema](https://fal.ai/api/openapi/queue/openapi.json?endpoint_id=fal-ai/kling-video/v3/4k/text-to-video)

### fal.ai Platform

- [Platform Documentation](https://docs.fal.ai)
- [Python Client](https://docs.fal.ai/clients/python)
- [JavaScript Client](https://docs.fal.ai/clients/javascript)
