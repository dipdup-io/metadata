CREATE OR REPLACE FUNCTION public.token_metadata_expired(IN p_item token_metadata)
    RETURNS boolean
    LANGUAGE 'plpgsql' STABLE
    PARALLEL SAFE
    COST 100
    
AS $BODY$
begin
    return to_timestamp(p_item.updated_at + 7200) < now();
end;
$BODY$;