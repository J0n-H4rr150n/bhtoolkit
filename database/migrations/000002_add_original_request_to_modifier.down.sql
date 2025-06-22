-- Revert adding original_request_headers and original_request_body to modifier_tasks
ALTER TABLE modifier_tasks DROP COLUMN original_request_headers;
ALTER TABLE modifier_tasks DROP COLUMN original_request_body;
ALTER TABLE modifier_tasks DROP COLUMN original_response_headers;
ALTER TABLE modifier_tasks DROP COLUMN original_response_body;
