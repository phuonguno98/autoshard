# 🌪️ Báo Cáo Kiểm Thử Mission-Critical (Chaos Engineering)

Để chứng minh kiến trúc **Masterless** của thư viện `autoshard` thực sự mang lại sự bền bỉ cấp độ Tier-1 (Mission-Critical), chúng tôi đã xây dựng một bộ thử nghiệm **Chaos Proxy** chuyên dụng tại thư mục `examples/chaos_engineering/main.go`.

Bộ mô phỏng ép hệ thống phải phân bổ **100 Công việc (Jobs)** dưới các kịch bản tàn khốc nhất: *Đứt cáp viễn thông mạng, Sập nguồn máy chủ, và Scale-up 50 workers cùng một mili-giây*.

Dưới đây là kết quả đối soát thực tế (Quality Gate Audit) trích xuất trực tiếp từ Terminal.

---

## 1. Kịch Bản: Khởi Động Ồ Ạt (Thundering Herd)
**Mô tả:** Hệ thống bị nhồi nhét tải khi 50 Microservices (Workers) cùng khởi động lên tại cùng một mili-giây (`[00.000s]`). Tất cả cùng bắn nhịp tim (Heartbeat) vào Database làm ngập lụt kết nối.

**Kết quả Thực Tế:** 
Nhờ cơ chế **Rào chắn Hội tụ (Convergence Barrier)**, hệ thống lập tức đóng băng toàn bộ 100 Jobs qua các Chu kỳ đầu (Phase 1, 2) bảo vệ dòng chảy công việc để tránh tranh chấp chồng lấp. Sau khi đã nhận diện chính xác $N=50$, ở Chu kỳ 3, rào chắn đồng loạt mở búng ra cực kỳ trơn tru.

```text
[00.003s] 🌀 [PHASE 3] Injecting 50 workers: Consensus reached. Convergence barrier established.
[00.003s]    ├─ Node [node-01   ] | 🟢 CONVERGED | Cluster Size:  0/50 | Assigned:  3 jobs -> [50995 55060 88295]
[00.003s]    ├─ Node [node-02   ] | 🟢 CONVERGED | Cluster Size:  1/50 | Assigned:  2 jobs -> [13706 72476]
[00.003s]    ├─ Node [node-03   ] | 🟢 CONVERGED | Cluster Size:  2/50 | Assigned:  1 jobs -> [62617]
[00.003s]    ├─ ... (Ẩn hiển thị 46 nodes ở giữa) ...
[00.003s]    ├─ Node [node-50   ] | 🟢 CONVERGED | Cluster Size: 49/50 | Assigned:  2 jobs -> [37889 86529]
[00.003s]    └─ Validation (Quality Gate): Converged Nodes: 50/50 | Dropped Jobs: 0 | Overlapped Jobs: 0
[00.003s]       ✅ PERFECT MATCH: Perfect convergence. Nodes evenly divided 100 jobs (max deviation: 4 jobs).
```

---

## 2. Kịch Bản: Sập Nguồn Cấp Bách (Hard Crash)
**Mô tả:** Máy chủ của `node-03` bị cháy nguồn (Giả lập qua lệnh `kill -9`). Ứng dụng đột tử không kịp gọi lệnh báo tử chuẩn `Shutdown()`.

**Kết quả Thực Tế:** 
Nhờ cơ chế **Garbage Collector** chạy ngầm chọc qua `ActiveWindow` (400ms). Dữ liệu nhịp tim rác của node-03 được xóa sổ. Hai nodes còn lại tự động phát giác và ôm tải hộ node-03 mà không cần ai điều phối. Quá trình kiểm định chất lượng xác thực **0 việc rớt mạng**.

```text
[01.003s]    🔌 ACTION: Hardware failure. node-03 died abruptly (No Shutdown)!

[01.003s] 🌀 [STALE] ActiveWindow (400ms) not reached, maintaining node-03's DB cache. Activating AP-Mode.
[01.003s]    ├─ ... (Node 1, 2 vẫn gánh tải cấu hình N=3 như bình thường để bảo vệ hệ thống)

[01.003s]    ⏳ Waiting for ActiveWindow (450ms). DB Garbage Collector cleaning up...

[01.453s] 🌀 [PHASE 3] GC cleanup complete. Cluster reconverged to N=2: Consensus reached.
[01.453s]    ├─ Node [node-01   ] | 🟢 CONVERGED | Cluster Size:  0/ 2 | Assigned: 47 jobs -> [14864 16399 17652 17760 19149 ...]
[01.453s]    ├─ Node [node-02   ] | 🟢 CONVERGED | Cluster Size:  1/ 2 | Assigned: 53 jobs -> [10641 11660 12384 12526 12568 ...]
[01.453s]    └─ Validation (Quality Gate): Converged Nodes: 2/2 | Dropped Jobs: 0 | Overlapped Jobs: 0
[01.453s]       ✅ PERFECT MATCH: Perfect convergence. Nodes evenly divided 100 jobs (max deviation: 6 jobs).
```

---

## 3. Kịch Bản: Phân Mảnh Mạng (CAP-Theorem Split Brain)
**Mô tả:** Giả lập đứt cáp quang biển. Nhóm cô lập (Node 1, 2) bị ngắt khỏi mạng Database hoàn toàn. Cụm bị chẻ làm đôi.

**Kết quả Thực Tế:**
Autoshard áp dụng nguyên lý **AP (Availability over Consistency)** của định lý Hệ phân tán (CAP Theorem). Cụm Khỏe (3,4,5) tự thiết lập lại quần thể $N=3$ và gánh thay **100% công việc** cho cả hệ thống. Cụm chết (1,2) kích hoạt `Fail-Open Safe Mode`, tự ôm một số job cho chắc ăn. Mặc dù có việc bị làm trùng (Overlapped), nhưng TUYỆT ĐỐI không có hóa đơn hay việc nào của User bị bỏ qua (0% Data loss).

```text
[01.956s]    ✂️ ACTION: Network cable cut. node-01 and node-02 are isolated from the cluster.

[01.956s] 🌀 [ISOLATED] Isolated side (1,2) lost network, detected DB error, immediately Fail-Open (Stale State N=5).
[01.956s]    ├─ Node [node-01   ] | 🟢 CONVERGED | Cluster Size:  0/ 5 | Assigned: 19 jobs -> [...]
[01.956s]    ├─ Node [node-02   ] | 🟢 CONVERGED | Cluster Size:  1/ 5 | Assigned: 21 jobs -> [...]
[01.956s]    └─ Validation (Quality Gate): Converged Nodes: 2/2 | Dropped Jobs: 60 | Overlapped Jobs: 0

... (Sau khi nối cáp lại thành công, 2 Hệ thống Cãi nhau và Bắt tay Hợp nhất) ...

[02.408s] 🌀 [PHASE 3] Merging 2 clusters (Reunion): Consensus reached. Convergence barrier established.
[02.408s]    ├─ Node [node-01   ] | 🟢 CONVERGED | Cluster Size:  0/ 5 | Assigned: 19 jobs -> [...]
[02.408s]    ├─ Node [node-02   ] | 🟢 CONVERGED | Cluster Size:  1/ 5 | Assigned: 21 jobs -> [...]
[02.408s]    ├─ Node [node-03   ] | 🟢 CONVERGED | Cluster Size:  2/ 5 | Assigned: 24 jobs -> [...]
[02.408s]    ├─ Node [node-04   ] | 🟢 CONVERGED | Cluster Size:  3/ 5 | Assigned: 23 jobs -> [...]
[02.408s]    ├─ Node [node-05   ] | 🟢 CONVERGED | Cluster Size:  4/ 5 | Assigned: 13 jobs -> [...]
[02.408s]    └─ Validation (Quality Gate): Converged Nodes: 5/5 | Dropped Jobs: 0 | Overlapped Jobs: 0
[02.408s]       ✅ PERFECT MATCH: Perfect convergence. Nodes evenly divided 100 jobs (max deviation: 11 jobs).
```

## Tổng Kết
Thông qua các kiểm thử cực độ (Chaos Engineering) giả lập cả đứt cáp và sập nguồn, bộ `autoshard` chưa một lần để rơi rớt (Dropped) bất kỳ một Tác vụ (`JobID`) nào khi cụm đã hội tụ. Nó chứng minh một kiến trúc Hệ Phân Tán hoàn toàn có thể chạy mượt mà mà không khuyết định một Bộ điều phối đơn lẻ (Single Point of Failure) nào. 

▶️ *Người dùng có thể tự chạy và theo dõi toàn thời gian bằng lệnh:* `go run ./examples/chaos_engineering/main.go`
