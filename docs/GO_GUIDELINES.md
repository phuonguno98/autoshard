# Autoshard Go Code Guidelines

> **Tài liệu Hướng dẫn Chuẩn mực Lập trình Golang cho Dự án Autoshard**
> *Áp dụng bắt buộc cho toàn bộ Repository `github.com/phuonguno98/autoshard`.*

Tài liệu này định hình **"Autoshard Way"** - một tư duy lập trình thư viện (Library-Oriented) sắc bén nhằm xây dựng một thư viện Zero-Communication Consensus hiệu năng siêu cao, không rác bộ nhớ (Zero-Allocation) và tương thích rộng rãi.

---

## 1. Bản Quyền & Giấy Phép (Mandatory Header)
Mọi file mã nguồn (`*.go`) được tạo ra đều phải bắt buộc có khối Comment chứa thông báo giấy phép MIT ở đầu file.
```go
// MIT License
//
// Copyright (c) 2026 phuonguno
//
// Permission is hereby granted, free of charge, to any person obtaining a copy...
```

---

## 2. Nguyên Tắc Cốt Lõi (General Principles - "The Autoshard Way")

1. **Không Phụ Thuộc (Zero-Dependencies trong Core):**
   * Mã nguồn trong thư mục gốc (`/core.go`, `/interfaces.go`) tuyệt đối không được import bất kỳ thư viện bên thứ 3 nào. Chỉ dùng Standard Library của Go.
2. **Sức mạnh Generics:**
   * Tận dụng tối đa sức mạnh của Golang Generics cho `IsMyJob` để hỗ trợ đa dạng mọi loại định dạng `JobID` (string, int64, uint, v.v.).
3. **Threading An Toàn Tuyệt Đối (Bulletproof Concurrency):**
   * Mọi Struct có State dùng chung (như `Partitioner` hoặc các bộ nhớ trong memory Adapter) đều phải được bảo vệ chặt chẽ bằng `sync.RWMutex` tránh Race Condition.
4. **Functional Options Pattern:**
   * Mở rộng API luôn sử dụng Functional Options Pattern (như `WithHasher`, `WithJitterPercent`) thay vì thay đổi Constructor Signatures trực tiếp gây Breaking Change cho người dùng Library.
5. **Ngôn ngữ Giao Tiếp:** Sử dụng **Tiếng Việt** trong các tài liệu (Markdown), sử dụng **Tiếng Anh** trong toàn bộ các file code, comment code.

---

## 3. GoDoc & Viết Chú Thích (Documentation & Comments)

*Lưu ý (Truthfulness Rule):* **Cấm để lại Comment Outdated**. Nếu chỉnh sửa Logic, phải cập nhật Comment tiếng Anh ngay lập tức.
* Mỗi Package (Core, Adapters) đều bắt buộc phải có riêng file `doc.go` chứa comment giải thích chức năng tổng quan của Package đó.

### 3.1. GoDoc cho Public API / Struct
* Mọi cấu trúc dữ liệu, Interface, và Hàm Public/Private phải có GoDoc với chữ cái đầu viết Hoa, là cụm/câu hoàn chỉnh, và kết thúc bằng dấu chấm (`.`).
* *Định dạng chuẩn:* Bắt đầu bằng tên hàm/cấu trúc. Luôn viết bằng Tiếng Anh.

**✅ Chuẩn:**
```go
// Partitioner distributes jobs across active members using modulo hashing
// and a zero-communication consensus barrier (perceived_version).
type Partitioner struct { ... }
```

### 3.2. Comment Trong Code (Inline)
* **Block Comments:** Thuyết minh luồng thuật toán lớn (như Barrier Check, Asymmetric Safe Mode).
* **Inline Comments:** Giải thích nhanh cho các đoạn logic tricky. Luôn viết bằng Tiếng Anh, viết Hoa chữ cái đầu của câu và không cần chấm câu.

---

## 4. Xử Lý Lỗi Tập Trung (Error Handling)

Mặc dù Autoshard là một Thư viện, việc quản lý lỗi chuẩn chỉnh vẫn là cốt lõi để thân thiện với Developers:

* **Không chặn Lỗi Cứng nhắc (Fail-Open):** Ví dụ như hàm `Sync()`, nếu Database timeout, Partitioner nên sử dụng trạng thái cũ (Asymmetric Safe Mode) và bọc lỗi đẩy lại cho Caller, thay vì tự ý làm sập/panic chương trình.
* **Bọc Ngữ Cảnh (Error Wrapping):** LUÔN LUÔN bọc (Wrap) báo lỗi bằng `fmt.Errorf("...: %w", err)` để Developer rành mạch nguồn gốc khi debug.
* **Error String (Chuỗi Thông Báo Lỗi):** Phải viết Thường (lowercase) hoàn toàn, KHÔNG có dấu chấm cuối câu !

### 4.1. Công Thức Error Message Thực Dụng
`fmt.Errorf("<action verb> <object>: %w", err)`

* ✅ `fmt.Errorf("execute garbage collection drop query: %w", err)`
* ✅ `fmt.Errorf("fetch active members from registry: %w", err)`
* ❌ `fmt.Errorf("failed to fetch: %w", err)`

---

## 5. Giới Hạn Logging (Zero Logging Library Rule)

**QUAN TRỌNG:** Vì `autoshard` là một Library (Thư viện Golang), tuyệt đối **KHÔNG ĐƯỢC** tự ý gọi lệnh in log (như `log.Printf`, `fmt.Println`, hay sử dụng `zap`/`logrus` bên trong Core code).
* Việc in Log trực tiếp ra Console sẽ làm rác Standard Output của ứng dụng Consumer (Bên mua/xài thư viện bạn).
* Mọi trạng thái bất thường **PHẢI** được trả về dưới dạng `error` tĩnh để Caller tự quyết định cách hành xử log trong hệ thống của họ.

---

## 6. Luồng Kiểm Soát Chất Lượng (Quality Gate & Review)

* Mọi Commit nên dùng format **Conventional Commits**: `feat:`, `fix:`, `refactor:`, `docs:`, `test:`
* Thư viện yêu cầu Coverage cực cao (đặc biệt là State Machine và Hashing).
* Phải vượt qua cờ `go test -race ./...` để khẳng định luồng xử lý RWMutex không bị Leak/Data Race dưới tải siêu lớn của hàng nghìn Goroutines chạy đồng thời gọi hàm `IsMyJob`.
