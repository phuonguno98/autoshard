# Changelog

Tất cả các thay đổi đáng chú ý của dự án này sẽ được ghi nhận tại file này.

Trạng thái phiên bản (Versioning) tuân thủ tiêu chuẩn [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-03-26

Đây là phiên bản phát hành chính thức đầu tiên (Production-Ready) dành cho các hệ thống Hệ Phân Tán (Distributed Systems) quy mô lớn. `autoshard` cung cấp cơ chế phân chia tải Masterless không giới hạn, đảm bảo không có điểm đen (Single Point of Failure).

### Thêm Mới (Added)
- Cấu trúc cốt lõi **Hexagonal Architecture** độc lập hoàn toàn với các gói lưu trữ ngoài.
- Thuật toán phân bổ **Modulo Hashing** kết hợp **Zero-Allocation**: Tận dụng Golang Generics (`~int`, `~string`) để tính toán không phân bổ bộ nhớ (No Heap Allocation).
- Cơ chế **Thundering Herd Shield**: Tích hợp Asymmetric Stand-by (Chế độ chờ bất đối xứng) và độ trễ Jitter ngẫu nhiên cản phá bão kết nối.
- Cơ chế **ActiveWindow & GC Election**: Tự động nhận diện node chết lâm sàng (Hard Crash) và dọn rác.
- **MySQL Adapter**: Giao tiếp bền bỉ và chốt chặn thời gian `NOW()` chống Split-brain do lệch giờ máy chủ.
- **Redis Adapter**: Hỗ trợ Redis & Redis Cluster hiệu năng cao với dọn rác phân tán bằng `TTL`.
- **Memory Adapter**: Chức năng Lock-striping dành cho Cluster tĩnh hoặc Unit Testing cực nhanh bằng `sync.RWMutex`.
- Bộ khung cảnh báo **Observability Hooks**: Khai báo `OnSyncError` và `OnStateChange` tương thích với Prometheus/Grafana.
- Môi trường Validation **Chaos Engineering Simulator** trong `examples/chaos_engineering` xác thực khả năng chống Split-brain & tính liên tục AP-Mode theo định lý CAP.

### Ổn định & Hiệu Năng (Stability & Performance)
- Cam kết 100% Passes đối với cờ kiểm tra luồng song song (`go test -race`).
- Băng thông phân bổ tiệm cận hiệu năng phần cứng gốc với độ trễ tính bằng nanoseconds (`O(1)` hash lookup).
- Kiểm thử Hỗn loạn (Chaos Validated) chứng minh đạt tỷ lệ **0% Bỏ sót Tác vụ (Missing) / 0% Trùng lặp (Overlapped)** ở trạng thái cụm Hội tụ hoàn toàn (Converged).
