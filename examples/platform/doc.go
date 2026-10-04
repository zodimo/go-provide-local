// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package platform provides seams for overriding system operations, such as
// reading the current time and fanning a batch of tasks out onto a caller
// supplied execution strategy.
//
// This package is adapted from the platform package of
// google.golang.org/adk/v2. In the adaptation the context seam is backed by
// github.com/zodimo/go-provide-local/plocal rather than context.WithValue, so
// the two seams are plocal.WithProvider call sites under typed resource keys.
// The adaptation deliberately drops ADK's UUID seam; no ID provider exists
// here.
//
// Two seams are provided:
//
//   - Time — WithTimeProvider installs a TimeProvider and Now reads the
//     current time through it. With no installed provider, Now falls back to
//     the wall clock (time.Now).
//   - Task fan-out — WithTaskRunner installs a TaskRunner and RunTasks
//     dispatches a batch of tasks through it. With no installed runner,
//     RunTasks fans the tasks out over one goroutine each.
//
// Providers are carried explicitly on a context.Context. Carrying the
// provider on the context (rather than in a package-level variable) also
// keeps it isolated to a single call tree, which is what makes concurrent
// runs with independent providers safe.
package platform
