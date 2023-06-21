CREATE OR REPLACE FUNCTION public.contract_metadata_failed(IN p_item contract_metadata)
    RETURNS boolean
    LANGUAGE 'plpgsql' STABLE
    PARALLEL SAFE
    COST 100
    
AS $BODY$
begin
    return to_timestamp(p_item.created_at + 10800) < now() and p_item.status = 2;
end;
$BODY$;