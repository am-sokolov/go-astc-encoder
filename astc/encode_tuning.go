package astc

type encoderTuning struct {
	modeLimit                     int
	maxPartitionCount             int
	partitionIndexLimit           [blockMaxPartitions + 1]int
	partitionCandidateLimit       [blockMaxPartitions + 1]int
	dualPlaneCorrelationThreshold float32
}

func encoderTuningFromConfig(cfg Config) encoderTuning {
	t := encoderTuning{
		modeLimit:                     int(cfg.TuneBlockModeLimit),
		maxPartitionCount:             int(cfg.TunePartitionCountLimit),
		dualPlaneCorrelationThreshold: cfg.Tune2PlaneEarlyOutLimitCorrelation,
	}
	t.partitionIndexLimit[2] = int(cfg.Tune2PartitionIndexLimit)
	t.partitionIndexLimit[3] = int(cfg.Tune3PartitionIndexLimit)
	t.partitionIndexLimit[4] = int(cfg.Tune4PartitionIndexLimit)
	t.partitionCandidateLimit[2] = int(cfg.Tune2PartitioningCandidateLimit)
	t.partitionCandidateLimit[3] = int(cfg.Tune3PartitioningCandidateLimit)
	t.partitionCandidateLimit[4] = int(cfg.Tune4PartitioningCandidateLimit)
	return t
}

func encoderTuningFor(quality EncodeQuality, texelCount int) encoderTuning {
	// Keep existing preset behavior for fastest/fast/medium to preserve regression fixtures.
	switch quality {
	case EncodeFastest:
		return encoderTuning{
			modeLimit:         1,
			maxPartitionCount: 1,
		}
	case EncodeFast:
		return encoderTuning{
			modeLimit:         8,
			maxPartitionCount: 1,
		}
	case EncodeMedium:
		t := encoderTuning{
			modeLimit:         24,
			maxPartitionCount: 2,
		}
		t.partitionIndexLimit[2] = 64
		t.partitionCandidateLimit[2] = 2
		return t
	}

	// Higher quality presets borrow search limits from the reference astcenc presets.
	// The reference uses different tuning tables depending on the block texel count.
	highBandwidth := texelCount > 0 && texelCount < 25
	midBandwidth := texelCount >= 25 && texelCount < 64
	lowBandwidth := texelCount >= 64

	switch quality {
	case EncodeThorough:
		t := encoderTuning{
			// Keep the existing block-mode limit for performance; the C++ preset uses ~94.
			modeLimit:         64,
			maxPartitionCount: 4,
		}
		t.partitionIndexLimit[2] = 82
		t.partitionCandidateLimit[2] = 3
		t.partitionIndexLimit[3] = 60
		t.partitionCandidateLimit[3] = 2
		t.partitionIndexLimit[4] = 30
		t.partitionCandidateLimit[4] = 2
		if highBandwidth {
			t.dualPlaneCorrelationThreshold = 0.97
		} else if midBandwidth {
			t.dualPlaneCorrelationThreshold = 0.95
		} else if lowBandwidth {
			t.dualPlaneCorrelationThreshold = 0.97
		}
		return t
	case EncodeVeryThorough:
		t := encoderTuning{
			modeLimit:         98,
			maxPartitionCount: 4,
		}
		t.partitionIndexLimit[2] = 256
		t.partitionCandidateLimit[2] = 8
		t.partitionIndexLimit[3] = 128
		t.partitionIndexLimit[4] = 64
		if lowBandwidth {
			t.partitionCandidateLimit[3] = 5
			t.partitionCandidateLimit[4] = 2
		} else if midBandwidth {
			t.partitionCandidateLimit[3] = 6
			t.partitionCandidateLimit[4] = 3
		} else {
			t.partitionCandidateLimit[3] = 6
			t.partitionCandidateLimit[4] = 4
		}
		if highBandwidth {
			t.dualPlaneCorrelationThreshold = 0.98
		} else if midBandwidth {
			t.dualPlaneCorrelationThreshold = 0.98
		} else if lowBandwidth {
			t.dualPlaneCorrelationThreshold = 0.98
		}
		return t
	case EncodeExhaustive:
		t := encoderTuning{
			modeLimit:         100,
			maxPartitionCount: 4,
		}
		if highBandwidth {
			t.partitionIndexLimit[2] = 512
			t.partitionIndexLimit[3] = 512
			t.partitionIndexLimit[4] = 512
		} else {
			// The reference reduces the index search limits for larger blocks.
			t.partitionIndexLimit[2] = 256
			t.partitionIndexLimit[3] = 256
			t.partitionIndexLimit[4] = 256
		}
		t.partitionCandidateLimit[2] = 8
		t.partitionCandidateLimit[3] = 8
		t.partitionCandidateLimit[4] = 8
		if highBandwidth {
			t.dualPlaneCorrelationThreshold = 0.99
		} else if midBandwidth {
			t.dualPlaneCorrelationThreshold = 0.99
		} else if lowBandwidth {
			t.dualPlaneCorrelationThreshold = 0.99
		}
		return t
	default:
		// Unknown presets behave like medium.
		return encoderTuningFor(EncodeMedium, texelCount)
	}
}
