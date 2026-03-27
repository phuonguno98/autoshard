# HƯỚNG DẪN CÀI ĐẶT CƠ SỞ DỮ LIỆU (DATABASE SETUP)

**Dự án:** Autoshard (github.com/phuonguno98/autoshard)
**Trạng thái:** Yêu cầu tiên quyết cho MySQL Adapter

Tài liệu này hướng dẫn cách cấu hình và tạo bảng cần thiết trong cơ sở dữ liệu để Autoshard có thể hoạt động ổn định trong mô hình Masterless Active-Active.

---

## 1. TỔNG QUAN

Autoshard sử dụng `Registry` làm nơi lưu trữ thông tin về các `Member` (Thành viên) đang hoạt động trong cụm. Đối với MySQL Adapter, chúng ta cần một bảng để theo dõi nhịp tim (`Heartbeat`) và số lượng thành viên mà mỗi node đang nhìn thấy (`perceived_version`).

**Lưu ý quan trọng:**
- Autoshard sử dụng hàm `NOW()` của chính Database để tính toán thời gian, giúp loại bỏ hoàn toàn hiện tượng lệch đồng hồ giữa các node (Clock Skew) dẫn đến Split-Brain.
- Bảng này mặc định tên là `autoshard_registry`.

---

## 2. SQL SCRIPT

Bạn có thể tìm thấy file SQL tại: `scripts/sql/mysql_schema.sql`

Hoặc chạy trực tiếp câu lệnh sau trong MySQL:

```sql
CREATE TABLE IF NOT EXISTS `autoshard_registry` (
    `member_id` VARCHAR(255) NOT NULL COMMENT 'Định danh duy nhất của thành viên trong cụm',
    `perceived_version` INT NOT NULL DEFAULT 0 COMMENT 'Số lượng thành viên hoạt động mà member này ghi nhận',
    `last_heartbeat` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Thời điểm cuối cùng member gửi nhịp tim',
    PRIMARY KEY (`member_id`),
    INDEX `idx_last_heartbeat` (`last_heartbeat`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

---

## 3. CỐI HÌNH TRONG GOLANG

Khi khởi tạo `Registry` trong mã nguồn Go, hãy đảm bảo rằng bạn truyền đúng `tableName` nếu bạn không sử dụng tên mặc định.

```go
db, _ := sql.Open("mysql", dsn)
registry, err := mysql.NewRegistry(db, "autoshard_registry")
if err != nil {
    log.Fatal(err)
}
```

---

## 4. TỐI ƯU HÓA ENGINE

- **InnoDB:** Khuyên dùng nhờ khả năng hỗ trợ Row-level Locking, giúp các thao tác `INSERT ... ON DUPLICATE KEY UPDATE` diễn ra mượt mà hơn khi số lượng Member tăng cao.
- **Index:** `idx_last_heartbeat` đóng vai trò cực kỳ quan trọng cho Leader Election và Garbage Collection (GC) để lọc các node đã chết mà không làm nghẽn bảng.
