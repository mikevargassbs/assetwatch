-- +goose Up
-- Repoints hardware_summary at the new site_installations site fields and
-- the unit's most recent delivery docket, now that logistics/waybill data
-- comes from delivery_dockets instead of the old shipments table.
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
    sh.site_name,
    sh.site_location,
    sh.deployment_team,
    sh.shipped_via,
    sh.waybill_number,
    sh.delivered_at               AS shipment_delivered_at,
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
LEFT JOIN shipments sh ON sh.hardware_unit_id = hu.id
LEFT JOIN site_installations si ON si.hardware_unit_id = hu.id
LEFT JOIN client_acceptances ca ON ca.hardware_unit_id = hu.id
LEFT JOIN defect_reports dr ON dr.hardware_unit_id = hu.id;
