// Package drivers imports all ASR provider packages to trigger their init()
// registration. Adding a new provider only requires adding one import line here.
package drivers

import (
	_ "gitee.com/AY77-OP/audio-talk-ai/asr/doubao"
	_ "gitee.com/AY77-OP/audio-talk-ai/asr/mimoasr"
	_ "gitee.com/AY77-OP/audio-talk-ai/asr/openairealtime"
	_ "gitee.com/AY77-OP/audio-talk-ai/asr/openaiwhisper"
	_ "gitee.com/AY77-OP/audio-talk-ai/asr/xfyuniat"
	_ "gitee.com/AY77-OP/audio-talk-ai/asr/xfyunlfasr"
	_ "gitee.com/AY77-OP/audio-talk-ai/asr/xfyunrtasr"
	_ "gitee.com/AY77-OP/audio-talk-ai/asr/xfyunrtasrstd"
	_ "gitee.com/AY77-OP/audio-talk-ai/asr/xfyunspark"
)
