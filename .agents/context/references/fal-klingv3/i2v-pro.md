# Kling Video v3 Image to Video [Pro]

> Kling 3.0 Pro: Top-tier image-to-video with cinematic visuals, fluid motion, and native audio generation, with custom element support.


## Overview

- **Endpoint**: `https://fal.run/fal-ai/kling-video/v3/pro/image-to-video`
- **Model ID**: `fal-ai/kling-video/v3/pro/image-to-video`
- **Category**: image-to-video
- **Kind**: inference
**Tags**: image-to-video



## Pricing

For every second of video you generated, you will be charged **$0.112** (audio off) or **$0.168** (audio on), if voice control is used while generating audio you will be charged **$0.196**. For example, a 5s video with audio on and voice control will cost **$0.98**

For more details, see [fal.ai pricing](https://fal.ai/pricing).

## API Information

This model can be used via our HTTP API or more conveniently via our client libraries.
See the input and output schema below, as well as the usage examples.


### Input Schema

The API accepts the following input parameters:


- **`prompt`** (`string`, _optional_):
  Text prompt for video generation. Either prompt or multi_prompt must be provided, but not both.
  - Examples: "The craftsman slowly examines the bowl, turning it gently in his weathered hands. His eyes reflect years of wisdom. Subtle smile forms on his face. Dust particles drift in warm light. Breathing motion, blinking eyes."

- **`multi_prompt`** (`list<KlingV3MultiPromptElement>`, _optional_):
  List of prompts for multi-shot video generation. If provided, divides the video into multiple shots.
  - Array of KlingV3MultiPromptElement
  - Examples: null

- **`start_image_url`** (`string`, _required_):
  URL of the image to be used for the video
  - Examples: "https://storage.googleapis.com/falserverless/example_inputs/kling-v3/pro-i2v/start_image.png"

- **`duration`** (`DurationEnum`, _optional_):
  The duration of the generated video in seconds Default value: `"5"`
  - Default: `"5"`
  - Options: `"3"`, `"4"`, `"5"`, `"6"`, `"7"`, `"8"`, `"9"`, `"10"`, `"11"`, `"12"`, `"13"`, `"14"`, `"15"`
  - Examples: "12"

- **`generate_audio`** (`boolean`, _optional_):
  Whether to generate native audio for the video. Supports Chinese and English voice output. Other languages are automatically translated to English. For English speech, use lowercase letters; for acronyms or proper nouns, use uppercase. Default value: `true`
  - Default: `true`

- **`end_image_url`** (`string`, _optional_):
  URL of the image to be used for the end of the video

- **`elements`** (`list<KlingV3ComboElementInput>`, _optional_):
  Elements (characters/objects) to include in the video. Each example can either be an image set (frontal + reference images) or a video. Reference in prompt as @Element1, @Element2, etc.
  - Array of KlingV3ComboElementInput
  - Examples: [{"frontal_image_url":"https://v3b.fal.media/files/b/0a8cfd5f/-kZL-ha3Iuelku5IHXC-A_glasses.png","reference_image_urls":["https://v3b.fal.media/files/b/0a8cfd62/psPCmzrD1y9vDgdyNfKAL_glasses_back.png"]},{"video_url":"https://v3b.fal.media/files/b/0a8cfd66/b03SOiQvKLlFx_jqdNZ9z_child_video.mp4"}]

- **`shot_type`** (`ShotTypeEnum`, _optional_):
  The type of multi-shot video generation. 'intelligent' lets the model automatically determine shot structure. Default value: `"customize"`
  - Default: `"customize"`
  - Options: `"customize"`, `"intelligent"`

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
{
  "start_image_url": "https://storage.googleapis.com/falserverless/example_inputs/kling-v3/pro-i2v/start_image.png"
}
```

**Full Example**:

```json
{
  "prompt": "The craftsman slowly examines the bowl, turning it gently in his weathered hands. His eyes reflect years of wisdom. Subtle smile forms on his face. Dust particles drift in warm light. Breathing motion, blinking eyes.",
  "multi_prompt": null,
  "start_image_url": "https://storage.googleapis.com/falserverless/example_inputs/kling-v3/pro-i2v/start_image.png",
  "duration": "12",
  "generate_audio": true,
  "elements": [
    {
      "frontal_image_url": "https://v3b.fal.media/files/b/0a8cfd5f/-kZL-ha3Iuelku5IHXC-A_glasses.png",
      "reference_image_urls": [
        "https://v3b.fal.media/files/b/0a8cfd62/psPCmzrD1y9vDgdyNfKAL_glasses_back.png"
      ]
    },
    {
      "video_url": "https://v3b.fal.media/files/b/0a8cfd66/b03SOiQvKLlFx_jqdNZ9z_child_video.mp4"
    }
  ],
  "shot_type": "customize",
  "negative_prompt": "blur, distort, and low quality",
  "cfg_scale": 0.5
}
```


### Output Schema

The API returns the following output format:

- **`video`** (`File`, _required_):
  The generated video
  - Examples: {"file_size":8431922,"content_type":"video/mp4","file_name":"out.mp4","url":"https://storage.googleapis.com/falserverless/example_outputs/kling-v3/pro-i2v/out.mp4"}



**Example Response**:

```json
{
  "video": {
    "file_size": 8431922,
    "content_type": "video/mp4",
    "file_name": "out.mp4",
    "url": "https://storage.googleapis.com/falserverless/example_outputs/kling-v3/pro-i2v/out.mp4"
  }
}
```


## Usage Examples

### cURL

```bash
curl --request POST \
  --url https://fal.run/fal-ai/kling-video/v3/pro/image-to-video \
  --header "Authorization: Key $FAL_KEY" \
  --header "Content-Type: application/json" \
  --data '{
     "start_image_url": "https://storage.googleapis.com/falserverless/example_inputs/kling-v3/pro-i2v/start_image.png"
   }'
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
    "fal-ai/kling-video/v3/pro/image-to-video",
    arguments={
        "start_image_url": "https://storage.googleapis.com/falserverless/example_inputs/kling-v3/pro-i2v/start_image.png"
    },
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

const result = await fal.subscribe("fal-ai/kling-video/v3/pro/image-to-video", {
  input: {
    start_image_url: "https://storage.googleapis.com/falserverless/example_inputs/kling-v3/pro-i2v/start_image.png"
  },
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

- [Model Playground](https://fal.ai/models/fal-ai/kling-video/v3/pro/image-to-video)
- [API Documentation](https://fal.ai/models/fal-ai/kling-video/v3/pro/image-to-video/api)
- [OpenAPI Schema](https://fal.ai/api/openapi/queue/openapi.json?endpoint_id=fal-ai/kling-video/v3/pro/image-to-video)

### fal.ai Platform

- [Platform Documentation](https://docs.fal.ai)
- [Python Client](https://docs.fal.ai/clients/python)
- [JavaScript Client](https://docs.fal.ai/clients/javascript)
