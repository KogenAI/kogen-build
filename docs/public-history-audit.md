# History audit

This is an owner-facing inventory for deciding what, if anything, to do about reachable history. No history was rewritten. Findings describe categories without reproducing the original material.

Traversal: `git rev-list --all`, followed by `git ls-tree -r` and `git cat-file blob` for each reachable commit tree. The boundary is refs available locally at the time of the audit; unreachable objects, remote-only refs and deleted content not present in those trees are not covered. The scan includes tracked files of every type and records each commit, file and category once. Pattern-based categorization can include public dependency references and historical test fixtures; owner review is needed.

| commit | file | category |
| --- | --- | --- |
| `060e7dd6e186f6fb2a7ef8f7dfd1aee240e93b1b` | `.kogen/intents/k-drop-optins-build/intent.md` | internal landing or delivery notes |
| `060e7dd6e186f6fb2a7ef8f7dfd1aee240e93b1b` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `060e7dd6e186f6fb2a7ef8f7dfd1aee240e93b1b` | `.kogen/intents/k-review-seam/intent.md` | internal landing or delivery notes |
| `060e7dd6e186f6fb2a7ef8f7dfd1aee240e93b1b` | `.kogen/intents/k-review-seam/intent.md` | private setup or private repository reference |
| `060e7dd6e186f6fb2a7ef8f7dfd1aee240e93b1b` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `060e7dd6e186f6fb2a7ef8f7dfd1aee240e93b1b` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `060e7dd6e186f6fb2a7ef8f7dfd1aee240e93b1b` | `.kogen/intents/k-state-goldens/intent.md` | internal landing or delivery notes |
| `060e7dd6e186f6fb2a7ef8f7dfd1aee240e93b1b` | `README.md` | historical noncanonical Kogen component URL |
| `060e7dd6e186f6fb2a7ef8f7dfd1aee240e93b1b` | `README.md` | private setup or private repository reference |
| `09b177ee4570fd8e9c84c150dd6c5f114a922fbe` | `.kogen/intents/k-drop-optins-build/intent.md` | internal landing or delivery notes |
| `09b177ee4570fd8e9c84c150dd6c5f114a922fbe` | `.kogen/intents/k-models-file-build/intent.md` | internal landing or delivery notes |
| `09b177ee4570fd8e9c84c150dd6c5f114a922fbe` | `.kogen/intents/k-models-file-build/intent.md` | machine-specific path or host identifier |
| `09b177ee4570fd8e9c84c150dd6c5f114a922fbe` | `.kogen/intents/k-models-file-build/intent.md` | private setup or private repository reference |
| `09b177ee4570fd8e9c84c150dd6c5f114a922fbe` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `09b177ee4570fd8e9c84c150dd6c5f114a922fbe` | `.kogen/intents/k-review-seam/intent.md` | internal landing or delivery notes |
| `09b177ee4570fd8e9c84c150dd6c5f114a922fbe` | `.kogen/intents/k-review-seam/intent.md` | private setup or private repository reference |
| `09b177ee4570fd8e9c84c150dd6c5f114a922fbe` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `09b177ee4570fd8e9c84c150dd6c5f114a922fbe` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `09b177ee4570fd8e9c84c150dd6c5f114a922fbe` | `.kogen/intents/k-state-goldens/intent.md` | internal landing or delivery notes |
| `09b177ee4570fd8e9c84c150dd6c5f114a922fbe` | `README.md` | historical noncanonical Kogen component URL |
| `09b177ee4570fd8e9c84c150dd6c5f114a922fbe` | `README.md` | private setup or private repository reference |
| `09b177ee4570fd8e9c84c150dd6c5f114a922fbe` | `features/steps/k_models_file_build/harness.js` | private setup or private repository reference |
| `1a8a18d75c1f5ecf8aa057c49551a9a98c342d7c` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `1a8a18d75c1f5ecf8aa057c49551a9a98c342d7c` | `README.md` | private setup or private repository reference |
| `1c9182c3359626d9177b7dc1dbf8dff7aeea22f6` | `.kogen/intents/k-build-cmd-entry/intent.md` | internal landing or delivery notes |
| `1c9182c3359626d9177b7dc1dbf8dff7aeea22f6` | `.kogen/intents/k-drop-optins-build/intent.md` | internal landing or delivery notes |
| `1c9182c3359626d9177b7dc1dbf8dff7aeea22f6` | `.kogen/intents/k-models-file-build/intent.md` | internal landing or delivery notes |
| `1c9182c3359626d9177b7dc1dbf8dff7aeea22f6` | `.kogen/intents/k-models-file-build/intent.md` | machine-specific path or host identifier |
| `1c9182c3359626d9177b7dc1dbf8dff7aeea22f6` | `.kogen/intents/k-models-file-build/intent.md` | private setup or private repository reference |
| `1c9182c3359626d9177b7dc1dbf8dff7aeea22f6` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `1c9182c3359626d9177b7dc1dbf8dff7aeea22f6` | `.kogen/intents/k-review-seam/intent.md` | internal landing or delivery notes |
| `1c9182c3359626d9177b7dc1dbf8dff7aeea22f6` | `.kogen/intents/k-review-seam/intent.md` | private setup or private repository reference |
| `1c9182c3359626d9177b7dc1dbf8dff7aeea22f6` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `1c9182c3359626d9177b7dc1dbf8dff7aeea22f6` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `1c9182c3359626d9177b7dc1dbf8dff7aeea22f6` | `.kogen/intents/k-state-goldens/intent.md` | internal landing or delivery notes |
| `1c9182c3359626d9177b7dc1dbf8dff7aeea22f6` | `README.md` | historical noncanonical Kogen component URL |
| `1c9182c3359626d9177b7dc1dbf8dff7aeea22f6` | `README.md` | private setup or private repository reference |
| `1c9182c3359626d9177b7dc1dbf8dff7aeea22f6` | `features/steps/k_models_file_build/harness.js` | private setup or private repository reference |
| `266f1653b333342230184e1088ff2ece77b6369e` | `.kogen/intents/k-drop-optins-build/intent.md` | internal landing or delivery notes |
| `266f1653b333342230184e1088ff2ece77b6369e` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `266f1653b333342230184e1088ff2ece77b6369e` | `.kogen/intents/k-review-seam/intent.md` | internal landing or delivery notes |
| `266f1653b333342230184e1088ff2ece77b6369e` | `.kogen/intents/k-review-seam/intent.md` | private setup or private repository reference |
| `266f1653b333342230184e1088ff2ece77b6369e` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `266f1653b333342230184e1088ff2ece77b6369e` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `266f1653b333342230184e1088ff2ece77b6369e` | `.kogen/intents/k-state-goldens/intent.md` | internal landing or delivery notes |
| `266f1653b333342230184e1088ff2ece77b6369e` | `README.md` | private setup or private repository reference |
| `269823f79196855604ffa19e566eaf95898a7a8c` | `README.md` | private setup or private repository reference |
| `3109cd105f750979d5d92b25945fde36891ab738` | `README.md` | private setup or private repository reference |
| `3af7e47a2d7d03d837f173cde38d6bca2dd3a8b2` | `.kogen/intents/k-drop-optins-build/intent.md` | internal landing or delivery notes |
| `3af7e47a2d7d03d837f173cde38d6bca2dd3a8b2` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `3af7e47a2d7d03d837f173cde38d6bca2dd3a8b2` | `.kogen/intents/k-review-seam/intent.md` | internal landing or delivery notes |
| `3af7e47a2d7d03d837f173cde38d6bca2dd3a8b2` | `.kogen/intents/k-review-seam/intent.md` | private setup or private repository reference |
| `3af7e47a2d7d03d837f173cde38d6bca2dd3a8b2` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `3af7e47a2d7d03d837f173cde38d6bca2dd3a8b2` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `3af7e47a2d7d03d837f173cde38d6bca2dd3a8b2` | `.kogen/intents/k-state-goldens/intent.md` | internal landing or delivery notes |
| `3af7e47a2d7d03d837f173cde38d6bca2dd3a8b2` | `README.md` | historical noncanonical Kogen component URL |
| `3af7e47a2d7d03d837f173cde38d6bca2dd3a8b2` | `README.md` | private setup or private repository reference |
| `44063cbd8eacd2bfe574c825cf421e248662feb6` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `44063cbd8eacd2bfe574c825cf421e248662feb6` | `README.md` | private setup or private repository reference |
| `45e455ef8c5be18512e8940a96d39ff0f8eb3d8a` | `.kogen/intents/k-drop-optins-build/intent.md` | internal landing or delivery notes |
| `45e455ef8c5be18512e8940a96d39ff0f8eb3d8a` | `.kogen/intents/k-models-file-build/intent.md` | internal landing or delivery notes |
| `45e455ef8c5be18512e8940a96d39ff0f8eb3d8a` | `.kogen/intents/k-models-file-build/intent.md` | machine-specific path or host identifier |
| `45e455ef8c5be18512e8940a96d39ff0f8eb3d8a` | `.kogen/intents/k-models-file-build/intent.md` | private setup or private repository reference |
| `45e455ef8c5be18512e8940a96d39ff0f8eb3d8a` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `45e455ef8c5be18512e8940a96d39ff0f8eb3d8a` | `.kogen/intents/k-review-seam/intent.md` | internal landing or delivery notes |
| `45e455ef8c5be18512e8940a96d39ff0f8eb3d8a` | `.kogen/intents/k-review-seam/intent.md` | private setup or private repository reference |
| `45e455ef8c5be18512e8940a96d39ff0f8eb3d8a` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `45e455ef8c5be18512e8940a96d39ff0f8eb3d8a` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `45e455ef8c5be18512e8940a96d39ff0f8eb3d8a` | `.kogen/intents/k-state-goldens/intent.md` | internal landing or delivery notes |
| `45e455ef8c5be18512e8940a96d39ff0f8eb3d8a` | `README.md` | historical noncanonical Kogen component URL |
| `45e455ef8c5be18512e8940a96d39ff0f8eb3d8a` | `README.md` | private setup or private repository reference |
| `45e455ef8c5be18512e8940a96d39ff0f8eb3d8a` | `features/steps/k_models_file_build/harness.js` | private setup or private repository reference |
| `4c14b9d1e7ff2113d18ae2f88ba230c11a0ec7e4` | `.kogen/intents/k-drop-optins-build/intent.md` | internal landing or delivery notes |
| `4c14b9d1e7ff2113d18ae2f88ba230c11a0ec7e4` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `4c14b9d1e7ff2113d18ae2f88ba230c11a0ec7e4` | `.kogen/intents/k-review-seam/intent.md` | internal landing or delivery notes |
| `4c14b9d1e7ff2113d18ae2f88ba230c11a0ec7e4` | `.kogen/intents/k-review-seam/intent.md` | private setup or private repository reference |
| `4c14b9d1e7ff2113d18ae2f88ba230c11a0ec7e4` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `4c14b9d1e7ff2113d18ae2f88ba230c11a0ec7e4` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `4c14b9d1e7ff2113d18ae2f88ba230c11a0ec7e4` | `.kogen/intents/k-state-goldens/intent.md` | internal landing or delivery notes |
| `4c14b9d1e7ff2113d18ae2f88ba230c11a0ec7e4` | `README.md` | historical noncanonical Kogen component URL |
| `4c14b9d1e7ff2113d18ae2f88ba230c11a0ec7e4` | `README.md` | private setup or private repository reference |
| `5140f0d0789b39395701f12ab58574d8d727e278` | `.kogen/intents/k-drop-optins-build/intent.md` | internal landing or delivery notes |
| `5140f0d0789b39395701f12ab58574d8d727e278` | `.kogen/intents/k-models-file-build/intent.md` | internal landing or delivery notes |
| `5140f0d0789b39395701f12ab58574d8d727e278` | `.kogen/intents/k-models-file-build/intent.md` | machine-specific path or host identifier |
| `5140f0d0789b39395701f12ab58574d8d727e278` | `.kogen/intents/k-models-file-build/intent.md` | private setup or private repository reference |
| `5140f0d0789b39395701f12ab58574d8d727e278` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `5140f0d0789b39395701f12ab58574d8d727e278` | `.kogen/intents/k-review-seam/intent.md` | internal landing or delivery notes |
| `5140f0d0789b39395701f12ab58574d8d727e278` | `.kogen/intents/k-review-seam/intent.md` | private setup or private repository reference |
| `5140f0d0789b39395701f12ab58574d8d727e278` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `5140f0d0789b39395701f12ab58574d8d727e278` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `5140f0d0789b39395701f12ab58574d8d727e278` | `.kogen/intents/k-state-goldens/intent.md` | internal landing or delivery notes |
| `5140f0d0789b39395701f12ab58574d8d727e278` | `README.md` | historical noncanonical Kogen component URL |
| `5140f0d0789b39395701f12ab58574d8d727e278` | `README.md` | private setup or private repository reference |
| `5140f0d0789b39395701f12ab58574d8d727e278` | `features/steps/k_models_file_build/harness.js` | private setup or private repository reference |
| `5495132a0cd583a73fefc66026e3167d4441cb40` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `5495132a0cd583a73fefc66026e3167d4441cb40` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `5495132a0cd583a73fefc66026e3167d4441cb40` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `5495132a0cd583a73fefc66026e3167d4441cb40` | `.kogen/intents/k-state-goldens/intent.md` | internal landing or delivery notes |
| `5495132a0cd583a73fefc66026e3167d4441cb40` | `README.md` | private setup or private repository reference |
| `5e08879bf2909aa1dd874bb37d71935c52e1a86b` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `5e08879bf2909aa1dd874bb37d71935c52e1a86b` | `README.md` | private setup or private repository reference |
| `65f9d77b4cc39a6452d810cef8303ef9a66fb272` | `README.md` | private setup or private repository reference |
| `77577264387a164f1ce1928fd414e852916cc6ed` | `.kogen/intents/k-drop-optins-build/intent.md` | internal landing or delivery notes |
| `77577264387a164f1ce1928fd414e852916cc6ed` | `.kogen/intents/k-models-file-build/intent.md` | internal landing or delivery notes |
| `77577264387a164f1ce1928fd414e852916cc6ed` | `.kogen/intents/k-models-file-build/intent.md` | machine-specific path or host identifier |
| `77577264387a164f1ce1928fd414e852916cc6ed` | `.kogen/intents/k-models-file-build/intent.md` | private setup or private repository reference |
| `77577264387a164f1ce1928fd414e852916cc6ed` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `77577264387a164f1ce1928fd414e852916cc6ed` | `.kogen/intents/k-review-seam/intent.md` | internal landing or delivery notes |
| `77577264387a164f1ce1928fd414e852916cc6ed` | `.kogen/intents/k-review-seam/intent.md` | private setup or private repository reference |
| `77577264387a164f1ce1928fd414e852916cc6ed` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `77577264387a164f1ce1928fd414e852916cc6ed` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `77577264387a164f1ce1928fd414e852916cc6ed` | `.kogen/intents/k-state-goldens/intent.md` | internal landing or delivery notes |
| `77577264387a164f1ce1928fd414e852916cc6ed` | `README.md` | historical noncanonical Kogen component URL |
| `77577264387a164f1ce1928fd414e852916cc6ed` | `README.md` | private setup or private repository reference |
| `77577264387a164f1ce1928fd414e852916cc6ed` | `features/steps/k_models_file_build/harness.js` | private setup or private repository reference |
| `7a9fac6cbd2b9017edd8a0aa5bc9bba5deb3dc60` | `README.md` | private setup or private repository reference |
| `7e660f0e19818f8fd6450bd964872ab2a8275749` | `README.md` | private setup or private repository reference |
| `92a5c7d4e650cdb9c4a729fcfd97f4ef9b99eff9` | `README.md` | private setup or private repository reference |
| `aea693e9c543b758f166e7d1868f586998b625db` | `.kogen/intents/k-drop-optins-build/intent.md` | internal landing or delivery notes |
| `aea693e9c543b758f166e7d1868f586998b625db` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `aea693e9c543b758f166e7d1868f586998b625db` | `.kogen/intents/k-review-seam/intent.md` | internal landing or delivery notes |
| `aea693e9c543b758f166e7d1868f586998b625db` | `.kogen/intents/k-review-seam/intent.md` | private setup or private repository reference |
| `aea693e9c543b758f166e7d1868f586998b625db` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `aea693e9c543b758f166e7d1868f586998b625db` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `aea693e9c543b758f166e7d1868f586998b625db` | `.kogen/intents/k-state-goldens/intent.md` | internal landing or delivery notes |
| `aea693e9c543b758f166e7d1868f586998b625db` | `README.md` | historical noncanonical Kogen component URL |
| `aea693e9c543b758f166e7d1868f586998b625db` | `README.md` | private setup or private repository reference |
| `b47cde70dcdf4fea58f2043b31619b07e8bdc9da` | `.kogen/intents/k-drop-optins-build/intent.md` | internal landing or delivery notes |
| `b47cde70dcdf4fea58f2043b31619b07e8bdc9da` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `b47cde70dcdf4fea58f2043b31619b07e8bdc9da` | `.kogen/intents/k-review-seam/intent.md` | internal landing or delivery notes |
| `b47cde70dcdf4fea58f2043b31619b07e8bdc9da` | `.kogen/intents/k-review-seam/intent.md` | private setup or private repository reference |
| `b47cde70dcdf4fea58f2043b31619b07e8bdc9da` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `b47cde70dcdf4fea58f2043b31619b07e8bdc9da` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `b47cde70dcdf4fea58f2043b31619b07e8bdc9da` | `.kogen/intents/k-state-goldens/intent.md` | internal landing or delivery notes |
| `b47cde70dcdf4fea58f2043b31619b07e8bdc9da` | `README.md` | private setup or private repository reference |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `.kogen/intents/k-baselineonce/intent.md` | internal landing or delivery notes |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `.kogen/intents/k-build-cmd-entry/intent.md` | internal landing or delivery notes |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `.kogen/intents/k-drop-optins-build/intent.md` | internal landing or delivery notes |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `.kogen/intents/k-models-file-build/intent.md` | internal landing or delivery notes |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `.kogen/intents/k-models-file-build/intent.md` | machine-specific path or host identifier |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `.kogen/intents/k-models-file-build/intent.md` | private setup or private repository reference |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `.kogen/intents/k-review-seam/intent.md` | internal landing or delivery notes |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `.kogen/intents/k-review-seam/intent.md` | private setup or private repository reference |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `.kogen/intents/k-state-goldens/intent.md` | internal landing or delivery notes |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `.kogen/project.yaml` | machine-specific path or host identifier |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `README.md` | historical noncanonical Kogen component URL |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `README.md` | private setup or private repository reference |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `features/steps/k_models_file_build/harness.js` | private setup or private repository reference |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `.kogen/intents/k-baselineonce/intent.md` | internal landing or delivery notes |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `.kogen/intents/k-build-cmd-entry/intent.md` | internal landing or delivery notes |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `.kogen/intents/k-drop-optins-build/intent.md` | internal landing or delivery notes |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `.kogen/intents/k-models-file-build/intent.md` | internal landing or delivery notes |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `.kogen/intents/k-models-file-build/intent.md` | machine-specific path or host identifier |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `.kogen/intents/k-models-file-build/intent.md` | private setup or private repository reference |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `.kogen/intents/k-review-seam/intent.md` | internal landing or delivery notes |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `.kogen/intents/k-review-seam/intent.md` | private setup or private repository reference |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `.kogen/intents/k-state-goldens/intent.md` | internal landing or delivery notes |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `README.md` | historical noncanonical Kogen component URL |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `README.md` | private setup or private repository reference |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `features/steps/k_models_file_build/harness.js` | private setup or private repository reference |
| `c8bc2fd3596816512d492ee4686f1a4358ee8b01` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `c8bc2fd3596816512d492ee4686f1a4358ee8b01` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `c8bc2fd3596816512d492ee4686f1a4358ee8b01` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `c8bc2fd3596816512d492ee4686f1a4358ee8b01` | `README.md` | private setup or private repository reference |
| `cfd80c29ce8d4b3fa92ab0425fd95c7ff72ab182` | `README.md` | private setup or private repository reference |
| `dfa857ff2689b795d197370af59b4801334254ab` | `.kogen/intents/k-drop-optins-build/intent.md` | internal landing or delivery notes |
| `dfa857ff2689b795d197370af59b4801334254ab` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `dfa857ff2689b795d197370af59b4801334254ab` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `dfa857ff2689b795d197370af59b4801334254ab` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `dfa857ff2689b795d197370af59b4801334254ab` | `.kogen/intents/k-state-goldens/intent.md` | internal landing or delivery notes |
| `dfa857ff2689b795d197370af59b4801334254ab` | `README.md` | private setup or private repository reference |
| `ef9549b0ef6cec3b79ebfc28b4ec1c4a867e301b` | `.kogen/intents/k-review-forced-no-submit/intent.md` | machine-specific path or host identifier |
| `ef9549b0ef6cec3b79ebfc28b4ec1c4a867e301b` | `.kogen/intents/k-spec-1-build/intent.md` | historical noncanonical Kogen component URL |
| `ef9549b0ef6cec3b79ebfc28b4ec1c4a867e301b` | `.kogen/intents/k-spec-1-build/intent.md` | internal landing or delivery notes |
| `ef9549b0ef6cec3b79ebfc28b4ec1c4a867e301b` | `README.md` | private setup or private repository reference |

| `060e7dd6e186f6fb2a7ef8f7dfd1aee240e93b1b` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `09b177ee4570fd8e9c84c150dd6c5f114a922fbe` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `1a8a18d75c1f5ecf8aa057c49551a9a98c342d7c` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `1c9182c3359626d9177b7dc1dbf8dff7aeea22f6` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `266f1653b333342230184e1088ff2ece77b6369e` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `3af7e47a2d7d03d837f173cde38d6bca2dd3a8b2` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `44063cbd8eacd2bfe574c825cf421e248662feb6` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `45e455ef8c5be18512e8940a96d39ff0f8eb3d8a` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `4c14b9d1e7ff2113d18ae2f88ba230c11a0ec7e4` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `5140f0d0789b39395701f12ab58574d8d727e278` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `5495132a0cd583a73fefc66026e3167d4441cb40` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `77577264387a164f1ce1928fd414e852916cc6ed` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `aea693e9c543b758f166e7d1868f586998b625db` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `b47cde70dcdf4fea58f2043b31619b07e8bdc9da` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `b8f6988f3b4aad5a44d645385ab30ffb31a97f21` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `b9990a3d090306d79e28ac0d858ba47c8f3b0ed4` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `c8bc2fd3596816512d492ee4686f1a4358ee8b01` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `dfa857ff2689b795d197370af59b4801334254ab` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |
| `ef9549b0ef6cec3b79ebfc28b4ec1c4a867e301b` | `.kogen/intents/k-golden-update-build/intent.md` | internal landing or delivery notes |

## Link verification

An anonymous redirect-following HTTP check of 94 distinct external URLs extracted from the tracked tree returned successful responses, as did the clone URL added to the README. Relative Markdown links were checked against the working tree. This verifies those targets at the time of the check, not their future availability.
