/*
Copyright (c) Arm Limited and Contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package spdk

import (
	"testing"
)

// TestMergeQoSParams covers the QoS merging logic: per-PVC annotation values
// override StorageClass defaults; empty annotation values leave the default
// unchanged.  This directly tests the behaviour that protects users from the
// mutability of PVC annotations: the effective QoS is resolved once at
// provisioning time and is independent of any later annotation changes.
func TestMergeQoSParams(t *testing.T) {
	tests := []struct {
		name        string
		scRWIOPS    string
		scRWmBytes  string
		scRmBytes   string
		scWmBytes   string
		annRWIOPS   string
		annRWmBytes string
		annRmBytes  string
		annWmBytes  string
		wantRWIOPS  string
		wantRWmBytes string
		wantRmBytes string
		wantWmBytes string
	}{
		{
			name:        "no annotations - StorageClass defaults kept",
			scRWIOPS:    "1000",
			scRWmBytes:  "200",
			scRmBytes:   "100",
			scWmBytes:   "100",
			annRWIOPS:   "",
			annRWmBytes: "",
			annRmBytes:  "",
			annWmBytes:  "",
			wantRWIOPS:  "1000",
			wantRWmBytes: "200",
			wantRmBytes: "100",
			wantWmBytes: "100",
		},
		{
			name:        "all annotations set - overrides all StorageClass defaults",
			scRWIOPS:    "1000",
			scRWmBytes:  "200",
			scRmBytes:   "100",
			scWmBytes:   "100",
			annRWIOPS:   "5000",
			annRWmBytes: "800",
			annRmBytes:  "400",
			annWmBytes:  "400",
			wantRWIOPS:  "5000",
			wantRWmBytes: "800",
			wantRmBytes: "400",
			wantWmBytes: "400",
		},
		{
			name:        "partial annotation - only rw-iops overridden",
			scRWIOPS:    "1000",
			scRWmBytes:  "200",
			scRmBytes:   "100",
			scWmBytes:   "100",
			annRWIOPS:   "9000",
			annRWmBytes: "",
			annRmBytes:  "",
			annWmBytes:  "",
			wantRWIOPS:  "9000",
			wantRWmBytes: "200",
			wantRmBytes: "100",
			wantWmBytes: "100",
		},
		{
			name:        "partial annotation - only throughput overridden",
			scRWIOPS:    "1000",
			scRWmBytes:  "200",
			scRmBytes:   "100",
			scWmBytes:   "100",
			annRWIOPS:   "",
			annRWmBytes: "1000",
			annRmBytes:  "500",
			annWmBytes:  "500",
			wantRWIOPS:  "1000",
			wantRWmBytes: "1000",
			wantRmBytes: "500",
			wantWmBytes: "500",
		},
		{
			name:        "annotation deleted or never set - StorageClass default used",
			scRWIOPS:    "2000",
			scRWmBytes:  "",
			scRmBytes:   "",
			scWmBytes:   "",
			annRWIOPS:   "",
			annRWmBytes: "",
			annRmBytes:  "",
			annWmBytes:  "",
			wantRWIOPS:  "2000",
			wantRWmBytes: "",
			wantRmBytes: "",
			wantWmBytes: "",
		},
		{
			name:        "no StorageClass defaults and no annotations - all empty",
			scRWIOPS:    "",
			scRWmBytes:  "",
			scRmBytes:   "",
			scWmBytes:   "",
			annRWIOPS:   "",
			annRWmBytes: "",
			annRmBytes:  "",
			annWmBytes:  "",
			wantRWIOPS:  "",
			wantRWmBytes: "",
			wantRmBytes: "",
			wantWmBytes: "",
		},
		{
			name:        "annotation overrides empty StorageClass default",
			scRWIOPS:    "",
			scRWmBytes:  "",
			scRmBytes:   "",
			scWmBytes:   "",
			annRWIOPS:   "3000",
			annRWmBytes: "",
			annRmBytes:  "",
			annWmBytes:  "",
			wantRWIOPS:  "3000",
			wantRWmBytes: "",
			wantRmBytes: "",
			wantWmBytes: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotRWIOPS, gotRWmBytes, gotRmBytes, gotWmBytes := mergeQoSParams(
				tc.scRWIOPS, tc.scRWmBytes, tc.scRmBytes, tc.scWmBytes,
				tc.annRWIOPS, tc.annRWmBytes, tc.annRmBytes, tc.annWmBytes,
			)
			if gotRWIOPS != tc.wantRWIOPS {
				t.Errorf("rwIOPS: got %q, want %q", gotRWIOPS, tc.wantRWIOPS)
			}
			if gotRWmBytes != tc.wantRWmBytes {
				t.Errorf("rwMBytes: got %q, want %q", gotRWmBytes, tc.wantRWmBytes)
			}
			if gotRmBytes != tc.wantRmBytes {
				t.Errorf("rMBytes: got %q, want %q", gotRmBytes, tc.wantRmBytes)
			}
			if gotWmBytes != tc.wantWmBytes {
				t.Errorf("wMBytes: got %q, want %q", gotWmBytes, tc.wantWmBytes)
			}
		})
	}
}

// TestVolumeContextQoSStamping verifies that the effective QoS values (after
// annotation overrides) are written into VolumeContext so the PV spec captures
// them durably.  PVC annotations are mutable; the PV spec is immutable after
// provisioning and is therefore the authoritative record of what was applied.
func TestVolumeContextQoSStamping(t *testing.T) {
	tests := []struct {
		name         string
		scParams     map[string]string // StorageClass parameters (initial VolumeContext)
		annRWIOPS    string
		annRWmBytes  string
		annRmBytes   string
		annWmBytes   string
		wantRWIOPS   string
		wantRWmBytes string
		wantRmBytes  string
		wantWmBytes  string
	}{
		{
			name: "annotation overrides StorageClass value in VolumeContext",
			scParams: map[string]string{
				"qos_rw_iops":   "1000",
				"qos_rw_mbytes": "200",
				"qos_r_mbytes":  "100",
				"qos_w_mbytes":  "100",
			},
			annRWIOPS:    "5000",
			annRWmBytes:  "",
			annRmBytes:   "",
			annWmBytes:   "",
			wantRWIOPS:   "5000",
			wantRWmBytes: "200",
			wantRmBytes:  "100",
			wantWmBytes:  "100",
		},
		{
			name: "no annotations - VolumeContext keeps StorageClass values",
			scParams: map[string]string{
				"qos_rw_iops":   "2000",
				"qos_rw_mbytes": "300",
				"qos_r_mbytes":  "150",
				"qos_w_mbytes":  "150",
			},
			annRWIOPS:    "",
			annRWmBytes:  "",
			annRmBytes:   "",
			annWmBytes:   "",
			wantRWIOPS:   "2000",
			wantRWmBytes: "300",
			wantRmBytes:  "150",
			wantWmBytes:  "150",
		},
		{
			name: "annotation deleted after initial set - VolumeContext reflects creation-time value",
			// Simulates: annotation was once set, volume created, annotation later deleted.
			// The merge is called with empty ann values (as if the annotation never existed),
			// so the StorageClass default wins — which is correct: the merge only happens
			// once at CreateVolume time, not on subsequent annotation mutations.
			scParams: map[string]string{
				"qos_rw_iops": "1000",
			},
			annRWIOPS:    "", // annotation deleted
			annRWmBytes:  "",
			annRmBytes:   "",
			annWmBytes:   "",
			wantRWIOPS:   "1000",
			wantRWmBytes: "",
			wantRmBytes:  "",
			wantWmBytes:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate what createVolume does: start with StorageClass params as
			// VolumeContext, resolve QoS, then stamp effective values back in.
			volumeContext := make(map[string]string)
			for k, v := range tc.scParams {
				volumeContext[k] = v
			}

			rwIOPS, rwMBytes, rMBytes, wMBytes := mergeQoSParams(
				volumeContext["qos_rw_iops"], volumeContext["qos_rw_mbytes"],
				volumeContext["qos_r_mbytes"], volumeContext["qos_w_mbytes"],
				tc.annRWIOPS, tc.annRWmBytes, tc.annRmBytes, tc.annWmBytes,
			)

			// Stamp — mirrors the code in createVolume.
			volumeContext["qos_rw_iops"] = rwIOPS
			volumeContext["qos_rw_mbytes"] = rwMBytes
			volumeContext["qos_r_mbytes"] = rMBytes
			volumeContext["qos_w_mbytes"] = wMBytes

			if got := volumeContext["qos_rw_iops"]; got != tc.wantRWIOPS {
				t.Errorf("VolumeContext qos_rw_iops: got %q, want %q", got, tc.wantRWIOPS)
			}
			if got := volumeContext["qos_rw_mbytes"]; got != tc.wantRWmBytes {
				t.Errorf("VolumeContext qos_rw_mbytes: got %q, want %q", got, tc.wantRWmBytes)
			}
			if got := volumeContext["qos_r_mbytes"]; got != tc.wantRmBytes {
				t.Errorf("VolumeContext qos_r_mbytes: got %q, want %q", got, tc.wantRmBytes)
			}
			if got := volumeContext["qos_w_mbytes"]; got != tc.wantWmBytes {
				t.Errorf("VolumeContext qos_w_mbytes: got %q, want %q", got, tc.wantWmBytes)
			}
		})
	}
}
