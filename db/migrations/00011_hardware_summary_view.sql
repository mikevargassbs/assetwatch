-- +goose Up
CREATE VIEW hardware_summary AS
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

-- +goose Down
DROP VIEW hardware_summary;
