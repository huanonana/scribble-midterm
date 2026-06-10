# Scribble Distributed

Scribble Distributed là phiên bản mở rộng của dự án mã nguồn mở
[sdomino/scribble](https://github.com/sdomino/scribble). Scribble gốc là một cơ
sở dữ liệu JSON nhúng viết bằng Go, lưu mỗi collection thành một thư mục và mỗi
resource thành một tệp JSON.

Dự án được thực hiện cho bài tập lớn về hệ phân tán. Nhóm giữ lại API lưu trữ
cục bộ của Scribble và bổ sung hai tính năng trao đổi dữ liệu giữa nhiều node.

## Hai tính năng mới

### 1. Sao chép dữ liệu giữa các node qua HTTP

Sau khi `Write` hoặc `Delete` thành công trên node hiện tại, thao tác được đóng
gói thành một `Operation` và gửi tới các peer đã cấu hình.

- Thao tác ghi và xóa mới được sao chép sang các node khác.
- Peer nhận thao tác chỉ áp dụng vào dữ liệu local, không gửi tiếp lần nữa.
- Cơ chế này ngăn vòng lặp replication giữa các node.
- Khi peer không phản hồi, node gọi thao tác nhận được lỗi replication.

### 2. Snapshot và đồng bộ node bằng `SyncFrom`

Một node có thể xuất toàn bộ trạng thái hiện tại qua endpoint snapshot. Node
mới hoặc node cần phục hồi sử dụng `SyncFrom` để tải và merge dữ liệu đó.

- Dùng để khởi tạo một node mới.
- Dùng để phục hồi dữ liệu từ một peer đang hoạt động.
- Snapshot được áp dụng local nên không tạo ra hàng loạt thao tác replication.

## Kiến trúc tổng quát

```text
Client
  |
  v
Node 1: ghi dữ liệu local
  |---- POST /scribble/v1/operations ----> Node 2
  |---- POST /scribble/v1/operations ----> Node 3

Node mới
  |---- GET /scribble/v1/snapshot -------> Node đang hoạt động
  `---- SyncFrom: merge dữ liệu vào local
```

Đây là prototype phục vụ học tập, chưa phải hệ thống consensus. Dự án chưa hỗ
trợ quorum, leader election hoặc tự động phát hiện xung đột ghi đồng thời.

## Cấu trúc dự án

```text
scribble.go                 API Scribble gốc và tích hợp replication
distributed.go              Operation, HTTP handler, snapshot và SyncFrom
distributed_test.go         Kiểm thử các tính năng phân tán
example/distributed/main.go Chương trình demo chạy nhiều node
deliverables/               Báo cáo và slide thuyết trình
```

## Yêu cầu cài đặt

- Go có hỗ trợ Go Modules
- Git
- PowerShell hoặc terminal tương đương

## Chạy kiểm thử

```powershell
go test ./...
go vet ./...
```

## Thực nghiệm replication với hai node

Mở terminal thứ nhất và chạy node phụ:

```powershell
go run ./example/distributed -id node-2 -addr :8082 -data ./demo/node-2
```

Mở terminal thứ hai và chạy node chính:

```powershell
go run ./example/distributed -id node-1 -addr :8081 -data ./demo/node-1 -peers http://localhost:8082
```

Ghi dữ liệu qua node 1:

```powershell
Invoke-RestMethod -Method Put `
  -Uri http://localhost:8081/notes/demo `
  -ContentType application/json `
  -Body '{"title":"Scribble Distributed","body":"Dữ liệu từ node-1"}'
```

Đọc dữ liệu đã được sao chép từ node 2:

```powershell
Invoke-RestMethod -Method Get -Uri http://localhost:8082/notes/demo
```

## Thực nghiệm snapshot với node thứ ba

```powershell
go run ./example/distributed `
  -id node-3 `
  -addr :8083 `
  -data ./demo/node-3 `
  -sync-from http://localhost:8081

Invoke-RestMethod -Method Get -Uri http://localhost:8083/notes/demo
```

## Ví dụ sử dụng API

```go
db, err := scribble.New("./data", &scribble.Options{
    NodeID: "node-1",
    Peers:  []string{"http://localhost:8082"},
})
if err != nil {
    log.Fatal(err)
}

http.ListenAndServe(":8081", db.HTTPHandler())

// Đồng bộ một node mới từ node đang hoạt động.
if err := db.SyncFrom("http://localhost:8081"); err != nil {
    log.Fatal(err)
}
```

## Tài liệu bài tập

- deliverables/Môn Ưng Dụng Phân Tán (1).pptx`: slide thuyết trình.

## Nguồn tham khảo và giấy phép

Dự án được phát triển dựa trên
[sdomino/scribble](https://github.com/sdomino/scribble) và tiếp tục sử dụng
giấy phép MIT trong tệp `LICENSE`.
