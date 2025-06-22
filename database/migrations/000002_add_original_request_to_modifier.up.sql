-- Add original_request_headers and original_request_body to modifier_tasks
ALTER TABLE modifier_tasks ADD COLUMN original_request_headers TEXT;
ALTER TABLE modifier_tasks ADD COLUMN original_request_body TEXT;
ALTER TABLE modifier_tasks ADD COLUMN original_response_headers TEXT;
ALTER TABLE modifier_tasks ADD COLUMN original_response_body TEXT;