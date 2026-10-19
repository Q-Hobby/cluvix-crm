package define

// EnvImportCustomerFile — env chứa đường dẫn TUYỆT ĐỐI tới file Excel trên server mà công cụ
// migrate khách khi onboard (`POST /api/import/customer/from-excel`) đọc vào. Endpoint KHÔNG nhận
// upload (cố ý: chỉ ops chạy tay, đặt file lên server rồi gọi). Rỗng ⇒ endpoint từ chối
// `import_file_not_configured`. Trước đây đường dẫn (kèm tên 1 khách) được gán cứng trong code.
const EnvImportCustomerFile = "IMPORT_CUSTOMER_FILE"
