INSERT INTO permissions (name, action, resource) VALUES
('Create Incident', 'create', 'incident'),
('Read Incident', 'read', 'incident'),
('Update Incident', 'update', 'incident'),
('Delete Incident', 'delete', 'incident');

INSERT INTO roles (name, category) VALUES
('admin', 'Administration'),
('operator', 'Operations'),
('viewer', 'General');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p WHERE r.name='admin';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p WHERE r.name='operator' AND p.action IN ('create','read');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p WHERE r.name='viewer' AND p.action='read';
