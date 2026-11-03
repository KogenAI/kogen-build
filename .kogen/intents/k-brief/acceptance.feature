Feature: Internal builder brief caps default to full
  Scenario: Unset native settings preserve the existing opening message
    Then the uncapped builder opening message matches its existing golden

  Scenario: Capped context preserves complete authority in the existing blocks
    # The local fixture has 500 Git-visible files and a 20480-byte README.md.
    Then the builder brief with caps 200 and 8192 preserves authority and reports 300 omitted entries and 12288 omitted bytes
