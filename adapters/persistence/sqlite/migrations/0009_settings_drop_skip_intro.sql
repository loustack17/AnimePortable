-- SPDX-License-Identifier: MPL-2.0

-- Removes the fixed-seconds skip-intro setting that migration 0008 added; the feature was
-- dropped. Every other settings value is kept.
ALTER TABLE settings DROP COLUMN skip_intro_seconds;
