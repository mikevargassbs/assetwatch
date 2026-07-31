-- +goose Up
-- The Reports summary/PDF/CSV showed blank Site and Device Model for units
-- that hadn't reached the device-config or installation stage yet, because
-- hardware_summary only read dc.device_model/si.site_name — it never fell
-- back to the values captured on the unit itself at creation time
-- (hu.device_model/hu.device_make/hu.allocated_branch).
CREATE OR REPLACE VIEW hardware_summary AS
SELECT
    hu.id                                    AS hardware_unit_id,
    hu.barcode,
    hu.status,
    hu.current_stage,
    hu.board_column,
    hu.is_exception,
    hu.allocated_branch,
    hu.created_at,
    COALESCE(dc.device_make, hu.device_make)   AS device_make,
    COALESCE(dc.device_model, hu.device_model) AS device_model,
    dc.serial_number,
    dc.mac_address::text          AS mac_address,
    dc.signed_off_at              AS device_config_signed_off_at,
    fc.firmware_version,
    fc.signed_off_at              AS firmware_signed_off_at,
    COALESCE(si.site_name, hu.allocated_branch) AS site_name,
    si.site_location,
    si.deployment_team,
    ld.shipped_via,
    ld.waybill_number,
    ld.received_at                AS shipment_delivered_at,
    si.installed_location,
    si.installed_height_m,
    si.signed_off_at              AS installation_signed_off_at,
    ca.signed_off_at              AS acceptance_signed_off_at,
    dr.defect_type,
    dr.replacement_status         AS defect_replacement_status,
    dr.declared_date              AS defect_declared_date
FROM hardware_units hu
LEFT JOIN device_configurations dc ON dc.hardware_unit_id = hu.id
LEFT JOIN firmware_configurations fc ON fc.hardware_unit_id = hu.id
LEFT JOIN site_installations si ON si.hardware_unit_id = hu.id
LEFT JOIN LATERAL (
    SELECT d.shipped_via, d.waybill_number, d.received_at
    FROM delivery_dockets d
    JOIN delivery_docket_items i ON i.docket_id = d.id
    WHERE i.hardware_unit_id = hu.id
    ORDER BY d.created_at DESC
    LIMIT 1
) ld ON true
LEFT JOIN client_acceptances ca ON ca.hardware_unit_id = hu.id
LEFT JOIN defect_reports dr ON dr.hardware_unit_id = hu.id;

-- +goose Down
CREATE OR REPLACE VIEW hardware_summary AS
SELECT
    hu.id                         AS hardware_unit_id,
    hu.barcode,
    hu.status,
    hu.current_stage,
    hu.board_column,
    hu.is_exception,
    hu.allocated_branch,
    hu.created_at,
    dc.device_make,
    dc.device_model,
    dc.serial_number,
    dc.mac_address::text          AS mac_address,
    dc.signed_off_at              AS device_config_signed_off_at,
    fc.firmware_version,
    fc.signed_off_at              AS firmware_signed_off_at,
    si.site_name,
    si.site_location,
    si.deployment_team,
    ld.shipped_via,
    ld.waybill_number,
    ld.received_at                AS shipment_delivered_at,
    si.installed_location,
    si.installed_height_m,
    si.signed_off_at              AS installation_signed_off_at,
    ca.signed_off_at              AS acceptance_signed_off_at,
    dr.defect_type,
    dr.replacement_status         AS defect_replacement_status,
    dr.declared_date              AS defect_declared_date
FROM hardware_units hu
LEFT JOIN device_configurations dc ON dc.hardware_unit_id = hu.id
LEFT JOIN firmware_configurations fc ON fc.hardware_unit_id = hu.id
LEFT JOIN site_installations si ON si.hardware_unit_id = hu.id
LEFT JOIN LATERAL (
    SELECT d.shipped_via, d.waybill_number, d.received_at
    FROM delivery_dockets d
    JOIN delivery_docket_items i ON i.docket_id = d.id
    WHERE i.hardware_unit_id = hu.id
    ORDER BY d.created_at DESC
    LIMIT 1
) ld ON true
LEFT JOIN client_acceptances ca ON ca.hardware_unit_id = hu.id
LEFT JOIN defect_reports dr ON dr.hardware_unit_id = hu.id;
