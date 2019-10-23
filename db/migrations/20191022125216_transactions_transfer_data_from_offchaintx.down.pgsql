CREATE FUNCTION cant_migrate_beyond_this_point() RETURNS VOID AS
$$
DECLARE
BEGIN
    RAISE EXCEPTION 'can not migrate beyond this point. If you need to migrate further, you need to write migration schemas to add the balance field to a user';
END ;
$$ LANGUAGE 'plpsql';

SELECT "cant_migrate_beyond_this_point"()
