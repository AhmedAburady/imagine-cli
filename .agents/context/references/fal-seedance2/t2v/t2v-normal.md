# Seedance 2.0 Text to Video API

> ByteDance's most advanced text-to-video model. Cinematic output with native audio, multi-shot editing, real-world physics, and director-level camera control.


## Overview

- **Endpoint**: `https://fal.run/bytedance/seedance-2.0/text-to-video`
- **Model ID**: `bytedance/seedance-2.0/text-to-video`
- **Category**: text-to-video
- **Kind**: inference
**Tags**: stylized, transform, lipsync



## Pricing

For every second of 720p video you generated, you will be charged **$0.3034/second** and for 1080p you will be charged **$0.682/second**. Your request will cost $0.014 per 1000 tokens. The number of tokens is given by (height of output video * width of output video * duration * 24) / 1024. 

For more details, see [fal.ai pricing](https://fal.ai/pricing).

## API Information

This model can be used via our HTTP API or more conveniently via our client libraries.
See the input and output schema below, as well as the usage examples.


### Input Schema

The API accepts the following input parameters:


- **`prompt`** (`string`, _required_):
  The text prompt used to generate the video
  - Examples: "An octopus finds a football in the ocean and excitedly calls its octopus friends to come and play. Cut scene to an octopus football game under the sea."

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
  --url https://fal.run/bytedance/seedance-2.0/text-to-video \
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
    "bytedance/seedance-2.0/text-to-video",
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

const result = await fal.subscribe("bytedance/seedance-2.0/text-to-video", {
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

- [Model Playground](https://fal.ai/models/bytedance/seedance-2.0/text-to-video)
- [API Documentation](https://fal.ai/models/bytedance/seedance-2.0/text-to-video/api)
- [OpenAPI Schema](https://fal.ai/api/openapi/queue/openapi.json?endpoint_id=bytedance/seedance-2.0/text-to-video)

### fal.ai Platform

- [Platform Documentation](https://docs.fal.ai)
- [Python Client](https://docs.fal.ai/clients/python)
- [JavaScript Client](https://docs.fal.ai/clients/javascript)
