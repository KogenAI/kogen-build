Feature: Native continuation evidence for a fleet worker
  This feature supplies evidence, never fleet authorization or delivery of a customer feature.
  Original request (retained verbatim):
  Provide the native Build continuation adapter that a factory worker can call on a stopped, still-owned Build. Its maintained owner is kogen-build. Offer a model-free prepare/readback through the library and standalone interface, then an execution path that checks the same evidence again under the ordinary native Build lock before delegating to supported resume. Reuse existing resume validation so the readback cannot disagree with native refusal on filesystem identity, approved contract, source baseline, staged or working tree, stage, or executor support. Expose the exact requested Build, host-bound checkout, candidate tree, approval/request and checkpoint identities, the roles/checks still required, and a specific held reason. Only classify states the existing native resume already supports. Preparation has no provider, reservation, checkout edit or queue effect. A plan supplies evidence to the caller; only the existing fleet's fresh fenced claim, host, attempt, request revision/hash and ordinary provider/finishing admission can authorize execution. A stale, mismatched or duplicate dispatch must preserve evidence and cause no additional execution. Keep policy, account credentials, provider selection, scheduling and landing in their existing owners.
  
  The consumer is the private fleet dispatcher and worker used for stack.menu's sponsors draft and other customers' saved Builds. At 01:37 UTC, controller commit 8e01250d9ba46b979b87f06fdd6aa52c14544b59 supplied a checked original-host refusal before transfer. Native inspect still reports potentially_available for actual EU Build ef17c79b even though its checkpoint saved inode 2804384 and the restored checkout has inode 2466484 on device 2049. Both point to the same path, request rev3, source 05cefb92 and exact original tree 1fdbec543200f5030b1f21f410cd306c2c1c0c69. A correct reusable adapter reports checkout identity mismatch, records no paid attempt and leaves checkout and claim untouched. Exercise also a valid in-place checkpoint for another repo/slug and a plan whose request or checkout changed between preparation and execution. Use native fixtures without live providers. The original sponsors row stays held, separate reconstruction item ceb83d70-d670-4f19-9b2b-88ec86b0c8dd retains the unusable-checkout case. Do not add a per-product allowlist, rebind inodes, reconstruct work, bypass gates, or mark the sponsors feature delivered.

  Scenario: Refuse the restored original-host checkout without effects
    Given native-continuation-worker-adapter: the native fixture harness is available
    Then native-continuation-worker-adapter: the adapter satisfies "identity" with this evidence
      """
      {"build":"ef17c79b","saved_inode":2804384,"restored_inode":2466484,"device":2049,"request_revision":"rev3","source":"05cefb92","original_tree":"1fdbec543200f5030b1f21f410cd306c2c1c0c69","controller_commit":"8e01250d9ba46b979b87f06fdd6aa52c14544b59","checked_at":"01:37 UTC","reconstruction_item":"ceb83d70-d670-4f19-9b2b-88ec86b0c8dd","customer":"stack.menu sponsors draft","sponsors_state":"held"}
      """

  Scenario: Prepare and continue a different saved Build in place
    Given native-continuation-worker-adapter: the native fixture harness is available
    Then native-continuation-worker-adapter: the adapter satisfies "valid" with this evidence
      """
      {"require_other_repo_and_slug":true,"required_roles":["build","review"],"required_checks":["test -f app.txt"],"prepare_provider_calls":0,"additional_duplicate_calls":0}
      """

  Scenario: Recheck prepared evidence and fresh dispatch under the native lock
    Given native-continuation-worker-adapter: the native fixture harness is available
    Then native-continuation-worker-adapter: the adapter satisfies "stale" with this evidence
      """
      {"mutations":["request_revision","request_hash","host","claim","attempt","approval","checkout","checkpoint"],"additional_provider_calls":0,"additional_runs":0}
      """

  Scenario: Agree with native resume refusal and supported states
    Given native-continuation-worker-adapter: the native fixture harness is available
    Then native-continuation-worker-adapter: the adapter satisfies "parity" with this evidence
      """
      {"cases":["identity","approval","baseline","staged","working","head","stage","executor","stat_refresh"],"additional_prepare_calls":0}
      """
