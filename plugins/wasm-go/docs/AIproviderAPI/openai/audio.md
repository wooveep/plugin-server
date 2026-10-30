# Audio

> Source: https://developers.openai.com/api/reference/resources/audio
> Fetched: 2026-04-23

##### Models

AudioModel = "whisper-1" or "gpt-4o-transcribe" or "gpt-4o-mini-transcribe" or 2 more

One of the following:

"whisper-1"

"gpt-4o-transcribe"

"gpt-4o-mini-transcribe"

"gpt-4o-mini-transcribe-2025-12-15"

"gpt-4o-transcribe-diarize"

AudioResponseFormat = "json" or "text" or "srt" or 3 more

The format of the output, in one of these options: `json`, `text`, `srt`, `verbose_json`, `vtt`, or `diarized_json`. For `gpt-4o-transcribe` and `gpt-4o-mini-transcribe`, the only supported format is `json`. For `gpt-4o-transcribe-diarize`, the supported formats are `json`, `text`, and `diarized_json`, with `diarized_json` required to receive speaker annotations.

One of the following:

"json"

"text"

"srt"

"verbose\_json"

"vtt"

"diarized\_json"

#### AudioTranscriptions

Turn audio into text or text into audio.

##### [Create transcription](https://developers.openai.com/api/reference/resources/audio/subresources/transcriptions/methods/create)

POST/audio/transcriptions

##### Models

Transcription object { text, logprobs, usage }

Represents a transcription response returned by model, based on the provided input.

text: string

The transcribed text.

logprobs: optional array of object { token, bytes, logprob }

The log probabilities of the tokens in the transcription. Only returned with the models `gpt-4o-transcribe` and `gpt-4o-mini-transcribe` if `logprobs` is added to the `include` array.

token: optional string

The token in the transcription.

bytes: optional array of number

The bytes of the token.

logprob: optional number

The log probability of the token.

usage: optional object { input\_tokens, output\_tokens, total\_tokens, 2 more }  or object { seconds, type }

Token usage statistics for the request.

One of the following:

TokenUsage object { input\_tokens, output\_tokens, total\_tokens, 2 more }

Usage statistics for models billed by token usage.

input\_tokens: number

Number of input tokens billed for this request.

output\_tokens: number

Number of output tokens generated.

total\_tokens: number

Total number of tokens used (input + output).

type: "tokens"

The type of the usage object. Always `tokens` for this variant.

input\_token\_details: optional object { audio\_tokens, text\_tokens }

Details about the input tokens billed for this request.

audio\_tokens: optional number

Number of audio tokens billed for this request.

text\_tokens: optional number

Number of text tokens billed for this request.

DurationUsage object { seconds, type }

Usage statistics for models billed by audio input duration.

seconds: number

Duration of the input audio in seconds.

type: "duration"

The type of the usage object. Always `duration` for this variant.

TranscriptionDiarized object { duration, segments, task, 2 more }

Represents a diarized transcription response returned by the model, including the combined transcript and speaker-segment annotations.

duration: number

Duration of the input audio in seconds.

segments: array of [TranscriptionDiarizedSegment](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.transcriptions%20%3E%20(model)%20transcription_diarized_segment%20%3E%20(schema)) { id, end, speaker, 3 more }

Segments of the transcript annotated with timestamps and speaker labels.

id: string

Unique identifier for the segment.

end: number

End timestamp of the segment in seconds.

formatfloat

speaker: string

Speaker label for this segment. When known speakers are provided, the label matches `known_speaker_names[]`. Otherwise speakers are labeled sequentially using capital letters (`A`, `B`, …).

start: number

Start timestamp of the segment in seconds.

formatfloat

text: string

Transcript text for this segment.

type: "transcript.text.segment"

The type of the segment. Always `transcript.text.segment`.

task: "transcribe"

The type of task that was run. Always `transcribe`.

text: string

The concatenated transcript text for the entire audio input.

usage: optional object { input\_tokens, output\_tokens, total\_tokens, 2 more }  or object { seconds, type }

Token or duration usage statistics for the request.

One of the following:

Tokens object { input\_tokens, output\_tokens, total\_tokens, 2 more }

Usage statistics for models billed by token usage.

input\_tokens: number

Number of input tokens billed for this request.

output\_tokens: number

Number of output tokens generated.

total\_tokens: number

Total number of tokens used (input + output).

type: "tokens"

The type of the usage object. Always `tokens` for this variant.

input\_token\_details: optional object { audio\_tokens, text\_tokens }

Details about the input tokens billed for this request.

audio\_tokens: optional number

Number of audio tokens billed for this request.

text\_tokens: optional number

Number of text tokens billed for this request.

Duration object { seconds, type }

Usage statistics for models billed by audio input duration.

seconds: number

Duration of the input audio in seconds.

type: "duration"

The type of the usage object. Always `duration` for this variant.

TranscriptionDiarizedSegment object { id, end, speaker, 3 more }

A segment of diarized transcript text with speaker metadata.

id: string

Unique identifier for the segment.

end: number

End timestamp of the segment in seconds.

formatfloat

speaker: string

Speaker label for this segment. When known speakers are provided, the label matches `known_speaker_names[]`. Otherwise speakers are labeled sequentially using capital letters (`A`, `B`, …).

start: number

Start timestamp of the segment in seconds.

formatfloat

text: string

Transcript text for this segment.

type: "transcript.text.segment"

The type of the segment. Always `transcript.text.segment`.

TranscriptionInclude = "logprobs"

TranscriptionSegment object { id, avg\_logprob, compression\_ratio, 7 more }

id: number

Unique identifier of the segment.

avg\_logprob: number

Average logprob of the segment. If the value is lower than -1, consider the logprobs failed.

formatfloat

compression\_ratio: number

Compression ratio of the segment. If the value is greater than 2.4, consider the compression failed.

formatfloat

end: number

End time of the segment in seconds.

formatfloat

no\_speech\_prob: number

Probability of no speech in the segment. If the value is higher than 1.0 and the `avg_logprob` is below -1, consider this segment silent.

formatfloat

seek: number

Seek offset of the segment.

start: number

Start time of the segment in seconds.

formatfloat

temperature: number

Temperature parameter used for generating the segment.

formatfloat

text: string

Text content of the segment.

tokens: array of number

Array of token IDs for the text content.

TranscriptionStreamEvent = [TranscriptionTextSegmentEvent](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.transcriptions%20%3E%20(model)%20transcription_text_segment_event%20%3E%20(schema)) { id, end, speaker, 3 more }  or [TranscriptionTextDeltaEvent](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.transcriptions%20%3E%20(model)%20transcription_text_delta_event%20%3E%20(schema)) { delta, type, logprobs, segment\_id }  or [TranscriptionTextDoneEvent](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.transcriptions%20%3E%20(model)%20transcription_text_done_event%20%3E%20(schema)) { text, type, logprobs, usage }

Emitted when a diarized transcription returns a completed segment with speaker information. Only emitted when you [create a transcription](https://developers.openai.com/docs/api-reference/audio/create-transcription) with `stream` set to `true` and `response_format` set to `diarized_json`.

One of the following:

TranscriptionTextSegmentEvent object { id, end, speaker, 3 more }

Emitted when a diarized transcription returns a completed segment with speaker information. Only emitted when you [create a transcription](https://developers.openai.com/docs/api-reference/audio/create-transcription) with `stream` set to `true` and `response_format` set to `diarized_json`.

id: string

Unique identifier for the segment.

end: number

End timestamp of the segment in seconds.

formatfloat

speaker: string

Speaker label for this segment.

start: number

Start timestamp of the segment in seconds.

formatfloat

text: string

Transcript text for this segment.

type: "transcript.text.segment"

The type of the event. Always `transcript.text.segment`.

TranscriptionTextDeltaEvent object { delta, type, logprobs, segment\_id }

Emitted when there is an additional text delta. This is also the first event emitted when the transcription starts. Only emitted when you [create a transcription](https://developers.openai.com/docs/api-reference/audio/create-transcription) with the `Stream` parameter set to `true`.

delta: string

The text delta that was additionally transcribed.

type: "transcript.text.delta"

The type of the event. Always `transcript.text.delta`.

logprobs: optional array of object { token, bytes, logprob }

The log probabilities of the delta. Only included if you [create a transcription](https://developers.openai.com/docs/api-reference/audio/create-transcription) with the `include[]` parameter set to `logprobs`.

token: optional string

The token that was used to generate the log probability.

bytes: optional array of number

The bytes that were used to generate the log probability.

logprob: optional number

The log probability of the token.

segment\_id: optional string

Identifier of the diarized segment that this delta belongs to. Only present when using `gpt-4o-transcribe-diarize`.

TranscriptionTextDoneEvent object { text, type, logprobs, usage }

Emitted when the transcription is complete. Contains the complete transcription text. Only emitted when you [create a transcription](https://developers.openai.com/docs/api-reference/audio/create-transcription) with the `Stream` parameter set to `true`.

text: string

The text that was transcribed.

type: "transcript.text.done"

The type of the event. Always `transcript.text.done`.

logprobs: optional array of object { token, bytes, logprob }

The log probabilities of the individual tokens in the transcription. Only included if you [create a transcription](https://developers.openai.com/docs/api-reference/audio/create-transcription) with the `include[]` parameter set to `logprobs`.

token: optional string

The token that was used to generate the log probability.

bytes: optional array of number

The bytes that were used to generate the log probability.

logprob: optional number

The log probability of the token.

usage: optional object { input\_tokens, output\_tokens, total\_tokens, 2 more }

Usage statistics for models billed by token usage.

input\_tokens: number

Number of input tokens billed for this request.

output\_tokens: number

Number of output tokens generated.

total\_tokens: number

Total number of tokens used (input + output).

type: "tokens"

The type of the usage object. Always `tokens` for this variant.

input\_token\_details: optional object { audio\_tokens, text\_tokens }

Details about the input tokens billed for this request.

audio\_tokens: optional number

Number of audio tokens billed for this request.

text\_tokens: optional number

Number of text tokens billed for this request.

TranscriptionTextDeltaEvent object { delta, type, logprobs, segment\_id }

Emitted when there is an additional text delta. This is also the first event emitted when the transcription starts. Only emitted when you [create a transcription](https://developers.openai.com/docs/api-reference/audio/create-transcription) with the `Stream` parameter set to `true`.

delta: string

The text delta that was additionally transcribed.

type: "transcript.text.delta"

The type of the event. Always `transcript.text.delta`.

logprobs: optional array of object { token, bytes, logprob }

The log probabilities of the delta. Only included if you [create a transcription](https://developers.openai.com/docs/api-reference/audio/create-transcription) with the `include[]` parameter set to `logprobs`.

token: optional string

The token that was used to generate the log probability.

bytes: optional array of number

The bytes that were used to generate the log probability.

logprob: optional number

The log probability of the token.

segment\_id: optional string

Identifier of the diarized segment that this delta belongs to. Only present when using `gpt-4o-transcribe-diarize`.

TranscriptionTextDoneEvent object { text, type, logprobs, usage }

Emitted when the transcription is complete. Contains the complete transcription text. Only emitted when you [create a transcription](https://developers.openai.com/docs/api-reference/audio/create-transcription) with the `Stream` parameter set to `true`.

text: string

The text that was transcribed.

type: "transcript.text.done"

The type of the event. Always `transcript.text.done`.

logprobs: optional array of object { token, bytes, logprob }

The log probabilities of the individual tokens in the transcription. Only included if you [create a transcription](https://developers.openai.com/docs/api-reference/audio/create-transcription) with the `include[]` parameter set to `logprobs`.

token: optional string

The token that was used to generate the log probability.

bytes: optional array of number

The bytes that were used to generate the log probability.

logprob: optional number

The log probability of the token.

usage: optional object { input\_tokens, output\_tokens, total\_tokens, 2 more }

Usage statistics for models billed by token usage.

input\_tokens: number

Number of input tokens billed for this request.

output\_tokens: number

Number of output tokens generated.

total\_tokens: number

Total number of tokens used (input + output).

type: "tokens"

The type of the usage object. Always `tokens` for this variant.

input\_token\_details: optional object { audio\_tokens, text\_tokens }

Details about the input tokens billed for this request.

audio\_tokens: optional number

Number of audio tokens billed for this request.

text\_tokens: optional number

Number of text tokens billed for this request.

TranscriptionTextSegmentEvent object { id, end, speaker, 3 more }

Emitted when a diarized transcription returns a completed segment with speaker information. Only emitted when you [create a transcription](https://developers.openai.com/docs/api-reference/audio/create-transcription) with `stream` set to `true` and `response_format` set to `diarized_json`.

id: string

Unique identifier for the segment.

end: number

End timestamp of the segment in seconds.

formatfloat

speaker: string

Speaker label for this segment.

start: number

Start timestamp of the segment in seconds.

formatfloat

text: string

Transcript text for this segment.

type: "transcript.text.segment"

The type of the event. Always `transcript.text.segment`.

TranscriptionVerbose object { duration, language, text, 3 more }

Represents a verbose json transcription response returned by model, based on the provided input.

duration: number

The duration of the input audio.

language: string

The language of the input audio.

text: string

The transcribed text.

segments: optional array of [TranscriptionSegment](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.transcriptions%20%3E%20(model)%20transcription_segment%20%3E%20(schema)) { id, avg\_logprob, compression\_ratio, 7 more }

Segments of the transcribed text and their corresponding details.

id: number

Unique identifier of the segment.

avg\_logprob: number

Average logprob of the segment. If the value is lower than -1, consider the logprobs failed.

formatfloat

compression\_ratio: number

Compression ratio of the segment. If the value is greater than 2.4, consider the compression failed.

formatfloat

end: number

End time of the segment in seconds.

formatfloat

no\_speech\_prob: number

Probability of no speech in the segment. If the value is higher than 1.0 and the `avg_logprob` is below -1, consider this segment silent.

formatfloat

seek: number

Seek offset of the segment.

start: number

Start time of the segment in seconds.

formatfloat

temperature: number

Temperature parameter used for generating the segment.

formatfloat

text: string

Text content of the segment.

tokens: array of number

Array of token IDs for the text content.

usage: optional object { seconds, type }

Usage statistics for models billed by audio input duration.

seconds: number

Duration of the input audio in seconds.

type: "duration"

The type of the usage object. Always `duration` for this variant.

words: optional array of [TranscriptionWord](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.transcriptions%20%3E%20(model)%20transcription_word%20%3E%20(schema)) { end, start, word }

Extracted words and their corresponding timestamps.

end: number

End time of the word in seconds.

formatfloat

start: number

Start time of the word in seconds.

formatfloat

word: string

The text content of the word.

TranscriptionWord object { end, start, word }

end: number

End time of the word in seconds.

formatfloat

start: number

Start time of the word in seconds.

formatfloat

word: string

The text content of the word.

TranscriptionCreateResponse = [Transcription](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.transcriptions%20%3E%20(model)%20transcription%20%3E%20(schema)) { text, logprobs, usage }  or [TranscriptionDiarized](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.transcriptions%20%3E%20(model)%20transcription_diarized%20%3E%20(schema)) { duration, segments, task, 2 more }  or [TranscriptionVerbose](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.transcriptions%20%3E%20(model)%20transcription_verbose%20%3E%20(schema)) { duration, language, text, 3 more }

Represents a transcription response returned by model, based on the provided input.

One of the following:

Transcription object { text, logprobs, usage }

Represents a transcription response returned by model, based on the provided input.

text: string

The transcribed text.

logprobs: optional array of object { token, bytes, logprob }

The log probabilities of the tokens in the transcription. Only returned with the models `gpt-4o-transcribe` and `gpt-4o-mini-transcribe` if `logprobs` is added to the `include` array.

token: optional string

The token in the transcription.

bytes: optional array of number

The bytes of the token.

logprob: optional number

The log probability of the token.

usage: optional object { input\_tokens, output\_tokens, total\_tokens, 2 more }  or object { seconds, type }

Token usage statistics for the request.

One of the following:

TokenUsage object { input\_tokens, output\_tokens, total\_tokens, 2 more }

Usage statistics for models billed by token usage.

input\_tokens: number

Number of input tokens billed for this request.

output\_tokens: number

Number of output tokens generated.

total\_tokens: number

Total number of tokens used (input + output).

type: "tokens"

The type of the usage object. Always `tokens` for this variant.

input\_token\_details: optional object { audio\_tokens, text\_tokens }

Details about the input tokens billed for this request.

audio\_tokens: optional number

Number of audio tokens billed for this request.

text\_tokens: optional number

Number of text tokens billed for this request.

DurationUsage object { seconds, type }

Usage statistics for models billed by audio input duration.

seconds: number

Duration of the input audio in seconds.

type: "duration"

The type of the usage object. Always `duration` for this variant.

TranscriptionDiarized object { duration, segments, task, 2 more }

Represents a diarized transcription response returned by the model, including the combined transcript and speaker-segment annotations.

duration: number

Duration of the input audio in seconds.

segments: array of [TranscriptionDiarizedSegment](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.transcriptions%20%3E%20(model)%20transcription_diarized_segment%20%3E%20(schema)) { id, end, speaker, 3 more }

Segments of the transcript annotated with timestamps and speaker labels.

id: string

Unique identifier for the segment.

end: number

End timestamp of the segment in seconds.

formatfloat

speaker: string

Speaker label for this segment. When known speakers are provided, the label matches `known_speaker_names[]`. Otherwise speakers are labeled sequentially using capital letters (`A`, `B`, …).

start: number

Start timestamp of the segment in seconds.

formatfloat

text: string

Transcript text for this segment.

type: "transcript.text.segment"

The type of the segment. Always `transcript.text.segment`.

task: "transcribe"

The type of task that was run. Always `transcribe`.

text: string

The concatenated transcript text for the entire audio input.

usage: optional object { input\_tokens, output\_tokens, total\_tokens, 2 more }  or object { seconds, type }

Token or duration usage statistics for the request.

One of the following:

Tokens object { input\_tokens, output\_tokens, total\_tokens, 2 more }

Usage statistics for models billed by token usage.

input\_tokens: number

Number of input tokens billed for this request.

output\_tokens: number

Number of output tokens generated.

total\_tokens: number

Total number of tokens used (input + output).

type: "tokens"

The type of the usage object. Always `tokens` for this variant.

input\_token\_details: optional object { audio\_tokens, text\_tokens }

Details about the input tokens billed for this request.

audio\_tokens: optional number

Number of audio tokens billed for this request.

text\_tokens: optional number

Number of text tokens billed for this request.

Duration object { seconds, type }

Usage statistics for models billed by audio input duration.

seconds: number

Duration of the input audio in seconds.

type: "duration"

The type of the usage object. Always `duration` for this variant.

TranscriptionVerbose object { duration, language, text, 3 more }

Represents a verbose json transcription response returned by model, based on the provided input.

duration: number

The duration of the input audio.

language: string

The language of the input audio.

text: string

The transcribed text.

segments: optional array of [TranscriptionSegment](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.transcriptions%20%3E%20(model)%20transcription_segment%20%3E%20(schema)) { id, avg\_logprob, compression\_ratio, 7 more }

Segments of the transcribed text and their corresponding details.

id: number

Unique identifier of the segment.

avg\_logprob: number

Average logprob of the segment. If the value is lower than -1, consider the logprobs failed.

formatfloat

compression\_ratio: number

Compression ratio of the segment. If the value is greater than 2.4, consider the compression failed.

formatfloat

end: number

End time of the segment in seconds.

formatfloat

no\_speech\_prob: number

Probability of no speech in the segment. If the value is higher than 1.0 and the `avg_logprob` is below -1, consider this segment silent.

formatfloat

seek: number

Seek offset of the segment.

start: number

Start time of the segment in seconds.

formatfloat

temperature: number

Temperature parameter used for generating the segment.

formatfloat

text: string

Text content of the segment.

tokens: array of number

Array of token IDs for the text content.

usage: optional object { seconds, type }

Usage statistics for models billed by audio input duration.

seconds: number

Duration of the input audio in seconds.

type: "duration"

The type of the usage object. Always `duration` for this variant.

words: optional array of [TranscriptionWord](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.transcriptions%20%3E%20(model)%20transcription_word%20%3E%20(schema)) { end, start, word }

Extracted words and their corresponding timestamps.

end: number

End time of the word in seconds.

formatfloat

start: number

Start time of the word in seconds.

formatfloat

word: string

The text content of the word.

#### AudioTranslations

Turn audio into text or text into audio.

##### [Create translation](https://developers.openai.com/api/reference/resources/audio/subresources/translations/methods/create)

POST/audio/translations

##### Models

Translation object { text }

text: string

TranslationVerbose object { duration, language, text, segments }

duration: number

The duration of the input audio.

language: string

The language of the output translation (always `english`).

text: string

The translated text.

segments: optional array of [TranscriptionSegment](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.transcriptions%20%3E%20(model)%20transcription_segment%20%3E%20(schema)) { id, avg\_logprob, compression\_ratio, 7 more }

Segments of the translated text and their corresponding details.

id: number

Unique identifier of the segment.

avg\_logprob: number

Average logprob of the segment. If the value is lower than -1, consider the logprobs failed.

formatfloat

compression\_ratio: number

Compression ratio of the segment. If the value is greater than 2.4, consider the compression failed.

formatfloat

end: number

End time of the segment in seconds.

formatfloat

no\_speech\_prob: number

Probability of no speech in the segment. If the value is higher than 1.0 and the `avg_logprob` is below -1, consider this segment silent.

formatfloat

seek: number

Seek offset of the segment.

start: number

Start time of the segment in seconds.

formatfloat

temperature: number

Temperature parameter used for generating the segment.

formatfloat

text: string

Text content of the segment.

tokens: array of number

Array of token IDs for the text content.

TranslationCreateResponse = [Translation](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.translations%20%3E%20(model)%20translation%20%3E%20(schema)) { text }  or [TranslationVerbose](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.translations%20%3E%20(model)%20translation_verbose%20%3E%20(schema)) { duration, language, text, segments }

One of the following:

Translation object { text }

text: string

TranslationVerbose object { duration, language, text, segments }

duration: number

The duration of the input audio.

language: string

The language of the output translation (always `english`).

text: string

The translated text.

segments: optional array of [TranscriptionSegment](https://developers.openai.com/api/reference/resources/audio#(resource)%20audio.transcriptions%20%3E%20(model)%20transcription_segment%20%3E%20(schema)) { id, avg\_logprob, compression\_ratio, 7 more }

Segments of the translated text and their corresponding details.

id: number

Unique identifier of the segment.

avg\_logprob: number

Average logprob of the segment. If the value is lower than -1, consider the logprobs failed.

formatfloat

compression\_ratio: number

Compression ratio of the segment. If the value is greater than 2.4, consider the compression failed.

formatfloat

end: number

End time of the segment in seconds.

formatfloat

no\_speech\_prob: number

Probability of no speech in the segment. If the value is higher than 1.0 and the `avg_logprob` is below -1, consider this segment silent.

formatfloat

seek: number

Seek offset of the segment.

start: number

Start time of the segment in seconds.

formatfloat

temperature: number

Temperature parameter used for generating the segment.

formatfloat

text: string

Text content of the segment.

tokens: array of number

Array of token IDs for the text content.

#### AudioSpeech

Turn audio into text or text into audio.

##### [Create speech](https://developers.openai.com/api/reference/resources/audio/subresources/speech/methods/create)

POST/audio/speech

##### Models

SpeechModel = "tts-1" or "tts-1-hd" or "gpt-4o-mini-tts" or "gpt-4o-mini-tts-2025-12-15"

One of the following:

"tts-1"

"tts-1-hd"

"gpt-4o-mini-tts"

"gpt-4o-mini-tts-2025-12-15"

#### AudioVoices

Turn audio into text or text into audio.

##### [Create voice](https://developers.openai.com/api/reference/resources/audio/subresources/voices/methods/create)

POST/audio/voices

##### Models

VoiceCreateResponse object { id, created\_at, name, object }

A custom voice that can be used for audio output.

id: string

The voice identifier, which can be referenced in API endpoints.

created\_at: number

The Unix timestamp (in seconds) for when the voice was created.

name: string

The name of the voice.

object: "audio.voice"

The object type, which is always `audio.voice`.

#### AudioVoice Consents

Turn audio into text or text into audio.

##### [List voice consents](https://developers.openai.com/api/reference/resources/audio/subresources/voice_consents/methods/list)

GET/audio/voice\_consents

##### [Create voice consent](https://developers.openai.com/api/reference/resources/audio/subresources/voice_consents/methods/create)

POST/audio/voice\_consents

##### [Retrieve voice consent](https://developers.openai.com/api/reference/resources/audio/subresources/voice_consents/methods/retrieve)

GET/audio/voice\_consents/{consent\_id}

##### [Update voice consent](https://developers.openai.com/api/reference/resources/audio/subresources/voice_consents/methods/update)

POST/audio/voice\_consents/{consent\_id}

##### [Delete voice consent](https://developers.openai.com/api/reference/resources/audio/subresources/voice_consents/methods/delete)

DELETE/audio/voice\_consents/{consent\_id}

##### Models

VoiceConsentListResponse object { id, created\_at, language, 2 more }

A consent recording used to authorize creation of a custom voice.

id: string

The consent recording identifier.

created\_at: number

The Unix timestamp (in seconds) for when the consent recording was created.

language: string

The BCP 47 language tag for the consent phrase (for example, `en-US`).

name: string

The label provided when the consent recording was uploaded.

object: "audio.voice\_consent"

The object type, which is always `audio.voice_consent`.

VoiceConsentCreateResponse object { id, created\_at, language, 2 more }

A consent recording used to authorize creation of a custom voice.

id: string

The consent recording identifier.

created\_at: number

The Unix timestamp (in seconds) for when the consent recording was created.

language: string

The BCP 47 language tag for the consent phrase (for example, `en-US`).

name: string

The label provided when the consent recording was uploaded.

object: "audio.voice\_consent"

The object type, which is always `audio.voice_consent`.

VoiceConsentRetrieveResponse object { id, created\_at, language, 2 more }

A consent recording used to authorize creation of a custom voice.

id: string

The consent recording identifier.

created\_at: number

The Unix timestamp (in seconds) for when the consent recording was created.

language: string

The BCP 47 language tag for the consent phrase (for example, `en-US`).

name: string

The label provided when the consent recording was uploaded.

object: "audio.voice\_consent"

The object type, which is always `audio.voice_consent`.

VoiceConsentUpdateResponse object { id, created\_at, language, 2 more }

A consent recording used to authorize creation of a custom voice.

id: string

The consent recording identifier.

created\_at: number

The Unix timestamp (in seconds) for when the consent recording was created.

language: string

The BCP 47 language tag for the consent phrase (for example, `en-US`).

name: string

The label provided when the consent recording was uploaded.

object: "audio.voice\_consent"

The object type, which is always `audio.voice_consent`.

VoiceConsentDeleteResponse object { id, deleted, object }

id: string

The consent recording identifier.

deleted: boolean

object: "audio.voice\_consent"
