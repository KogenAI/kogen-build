Feature: Fit optional builder file inventory in the rendered request
  This feature uses synthetic local kogen-build runs and a scripted loopback provider. The historical byte figures identify failure shapes, not private fixtures or live requests.

  Background:
    Given build-fit-optional-file-inventory: the local native Build fixture is available

  Scenario: Brand migration opening
    When build-fit-optional-file-inventory: I replay "Brand" with 532421 bytes against the 524288-byte rendered ceiling
    Then build-fit-optional-file-inventory: the opening fits without losing authority or required paths and the omitted path remains discoverable

  Scenario: Product Room reset
    When build-fit-optional-file-inventory: I replay "ProductRoom" with 513246 bytes against the 524288-byte rendered ceiling
    Then build-fit-optional-file-inventory: the reset fits before shortening the newest outputs and keeps the exact archive and authority

  Scenario: Small listing stays verbatim
    When build-fit-optional-file-inventory: I replay "Small" at the 524288-byte rendered ceiling
    Then build-fit-optional-file-inventory: the opening keeps the existing ordered listing and exact authority

  Scenario: Irreducible authority retains evidence
    When build-fit-optional-file-inventory: I replay "Authority" at the 524288-byte rendered ceiling
    Then build-fit-optional-file-inventory: the ceiling and its adjacent values admit fitting authority and refuse irreducible authority with recoverable evidence
