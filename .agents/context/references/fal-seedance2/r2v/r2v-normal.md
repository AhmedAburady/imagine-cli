# Seedance 2 Reference to Video

> ByteDance's most advanced reference-to-video model. Generate video from up to 9 images, 3 videos, and 3 audio clips with native audio and cinematic camera control.


## Overview

- **Endpoint**: `https://fal.run/bytedance/seedance-2.0/reference-to-video`
- **Model ID**: `bytedance/seedance-2.0/reference-to-video`
- **Category**: image-to-video
- **Kind**: inference
**Tags**: stylized, transform, lipsync



## Pricing

For every second of 720p video you generated, you will be charged **$0.3034/second** and for 1080p you will be charged **$0.682/second**. Your request will cost $0.014 per 1000 tokens. The number of tokens is given by (height of output video * width of output video * duration * 24) / 1024.  **If video inputs are provided the price is multiplied by 0.6**. With video inputs and 720p resolution the price is **$0.1814** per second.

For more details, see [fal.ai pricing](https://fal.ai/pricing).

## API Information

This model can be used via our HTTP API or more conveniently via our client libraries.
See the input and output schema below, as well as the usage examples.


### Input Schema

The API accepts the following input parameters:


- **`prompt`** (`string`, _required_):
  The text prompt used to generate the video.
  - Examples: "An octopus finds a football in the ocean and excitedly calls its octopus friends to come and play. Cut scene to an octopus football game under the sea."

- **`image_urls`** (`list<string>`, _optional_):
  Reference images to guide video generation. Refer to them in the prompt as @Image1, @Image2, etc. Supported formats: JPEG, PNG, WebP. Max 30 MB per image. Up to 9 images. Total files across all modalities must not exceed 12.
  - Array of string
  - Examples: ["https://v3b.fal.media/files/b/0a8eba37/Cqg-4Uwzyz4DELfceT1CF_a17e588773ec45b1a9e6f100a787b80b.jpg"]

- **`video_urls`** (`list<string>`, _optional_):
  Reference videos to guide video generation. Refer to them in the prompt as @Video1, @Video2, etc. Supported formats: MP4, MOV. Up to 3 videos, combined duration must be between 2 and 15 seconds, total size under 50 MB. Each video must be between ~480p (640x640) and ~720p (834x1112) in resolution.
  - Array of string

- **`audio_urls`** (`list<string>`, _optional_):
  Reference audio to guide video generation. Refer to them in the prompt as @Audio1, @Audio2, etc. Supported formats: MP3, WAV. Up to 3 files, combined duration must not exceed 15 seconds. Max 15 MB per file.If audio is provided, at least one reference image or video is required.
  - Array of string

- **`resolution`** (`ResolutionEnum`, _optional_):
  Video resolution - 480p for faster generation, 720p for balance, 1080p for highest quality. Default value: `"720p"`
  - Default: `"720p"`
  - Options: `"480p"`, `"720p"`, `"1080p"`

- **`duration`** (`DurationEnum`, _optional_):
  Duration of the video in seconds. Supports 4 to 15 seconds, or auto to let the model decide based on the prompt. Default value: `"auto"`
  - Default: `"auto"`
  - Options: `"auto"`, `"4"`, `"5"`, `"6"`, `"7"`, `"8"`, `"9"`, `"10"`, `"11"`, `"12"`, `"13"`, `"14"`, `"15"`

- **`aspect_ratio`** (`AspectRatioEnum`, _optional_):
  The aspect ratio of the generated video. Use 16:9 for landscape, 9:16 for portrait/vertical, 1:1 for square, 21:9 for ultrawide cinematic, or auto to let the model decide. Default value: `"auto"`
  - Default: `"auto"`
  - Options: `"auto"`, `"21:9"`, `"16:9"`, `"4:3"`, `"1:1"`, `"3:4"`, `"9:16"`

- **`generate_audio`** (`boolean`, _optional_):
  Whether to generate synchronized audio for the video, including sound effects, ambient sounds, and lip-synced speech. The cost of video generation is the same regardless of whether audio is generated or not. Default value: `true`
  - Default: `true`

- **`bitrate_mode`** (`BitrateModeEnum`, _optional_):
  Output bitrate mode. 'high' requests a higher-quality, larger-file encode from the model; 'standard' uses the default bitrate. Default value: `"standard"`
  - Default: `"standard"`
  - Options: `"standard"`, `"high"`
  - Examples: "standard"

- **`seed`** (`integer`, _optional_):
  Random seed for reproducibility. Note that results may still vary slightly even with the same seed.

- **`end_user_id`** (`string`, _optional_):
  The unique user ID of the end user.



**Required Parameters Example**:

```json
{
  "prompt": "An octopus finds a football in the ocean and excitedly calls its octopus friends to come and play. Cut scene to an octopus football game under the sea."
}
```

**Full Example**:

```json
{
  "prompt": "An octopus finds a football in the ocean and excitedly calls its octopus friends to come and play. Cut scene to an octopus football game under the sea.",
  "image_urls": [
    "https://v3b.fal.media/files/b/0a8eba37/Cqg-4Uwzyz4DELfceT1CF_a17e588773ec45b1a9e6f100a787b80b.jpg"
  ],
  "resolution": "720p",
  "duration": "auto",
  "aspect_ratio": "auto",
  "generate_audio": true,
  "bitrate_mode": "standard"
}
```


### Output Schema

The API returns the following output format:

- **`video`** (`File`, _required_):
  The generated video file.
  - Examples: {"url":"https://storage.googleapis.com/falserverless/example_outputs/bytedance/seedance_2/output.mp4"}

- **`seed`** (`integer`, _required_):
  The seed used for generation.
  - Examples: 42



**Example Response**:

```json
{
  "video": {
    "url": "https://storage.googleapis.com/falserverless/example_outputs/bytedance/seedance_2/output.mp4"
  },
  "seed": 42
}
```


## Usage Examples

### cURL

```bash
curl --request POST \
  --url https://fal.run/bytedance/seedance-2.0/reference-to-video \
  --header "Authorization: Key $FAL_KEY" \
  --header "Content-Type: application/json" \
  --data '{
     "prompt": "An octopus finds a football in the ocean and excitedly calls its octopus friends to come and play. Cut scene to an octopus football game under the sea."
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
    "bytedance/seedance-2.0/reference-to-video",
    arguments={
        "prompt": "An octopus finds a football in the ocean and excitedly calls its octopus friends to come and play. Cut scene to an octopus football game under the sea."
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

const result = await fal.subscribe("bytedance/seedance-2.0/reference-to-video", {
  input: {
    prompt: "An octopus finds a football in the ocean and excitedly calls its octopus friends to come and play. Cut scene to an octopus football game under the sea."
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

- [Model Playground](https://fal.ai/models/bytedance/seedance-2.0/reference-to-video)
- [API Documentation](https://fal.ai/models/bytedance/seedance-2.0/reference-to-video/api)
- [OpenAPI Schema](https://fal.ai/api/openapi/queue/openapi.json?endpoint_id=bytedance/seedance-2.0/reference-to-video)

### fal.ai Platform

- [Platform Documentation](https://docs.fal.ai)
- [Python Client](https://docs.fal.ai/clients/python)
- [JavaScript Client](https://docs.fal.ai/clients/javascript)
