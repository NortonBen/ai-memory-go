// Example: Building and traversing a Knowledge Graph with ai-memory-brain and LM Studio
// Run: go run ./examples/knowledge_graph_builder/
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/NortonBen/ai-memory-go/engine"
	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
)

func main() {
	ctx := context.Background()
	// os.RemoveAll("./data/news")
	_ = os.MkdirAll("./data/news", 0o750)

	// ─── 1. LM Studio Embedder ────────────────────────────────────────────────
	lmstudioEmb := vector.NewLMStudioEmbeddingProvider("http://localhost:1234/v1", "text-embedding-nomic-embed-text-v1.5")
	cache := vector.NewInMemoryEmbeddingCache()
	embedder := vector.NewAutoEmbedder("lmstudio", cache)
	embedder.AddProvider("lmstudio", lmstudioEmb)

	// ─── 2. SQLite Stores (Graph, Vector, Relational) ─────────────────────────
	graphStore, err := graph.NewSQLiteGraphStore("./data/news/memory_graph.db")
	must(err, "graph store")
	defer graphStore.Close()

	vecStore, err := vector.NewSQLiteVectorStore("./data/news/memory_vectors.db", 768)
	must(err, "vector store")
	defer vecStore.Close()

	relStore, err := storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Database:    "./data/news/memory_rel.db",
		ConnTimeout: 5 * time.Second,
	})
	must(err, "rel store")
	defer relStore.Close()

	// ─── 3. LM Studio Extractor ───────────────────────────────────────────────
	lmstudioProvider, err := extractor.NewLMStudioProvider("http://localhost:1234/v1", "qwen/qwen3-4b-2507")
	must(err, "lmstudio provider")
	llmExt := extractor.NewBasicExtractor(lmstudioProvider, nil)

	// ─── 4. Memory Engine ─────────────────────────────────────────────────────
	// We'll use 2 workers for extraction
	eng := engine.NewMemoryEngineWithStores(llmExt, embedder, relStore, graphStore, vecStore, engine.EngineConfig{MaxWorkers: 2})
	defer eng.Close()

	sessionID := "kg-session-001"

	// ─── 5. Knowledge Corpus ──────────────────────────────────────────────────
	fmt.Println("=== INGESTING KNOWLEDGE CORPUS ===")
	corpus := []string{
		`
		Quan chức cấp cao Iran nêu rõ những điều kiện để chấm dứt cuộc chiến với Mỹ và Israel, đồng thời cảnh báo đáp trả các cuộc tấn công vào hạ tầng năng lượng.
Iran nêu loạt điều kiện với Mỹ - Israel - 1
Ông Mohsen Rezaei, thành viên Hội đồng Phân định Lợi ích Quốc gia Iran (Ảnh: PressTV).

Trong một cuộc phỏng vấn trên truyền hình hôm 24/3, ông Mohsen Rezaei, thành viên Hội đồng Phân định Lợi ích Quốc gia Iran và cựu chỉ huy Lực lượng Vệ binh Cách mạng Hồi giáo (IRGC), đã nêu một loạt điều kiện để chấm dứt cuộc xung đột giữa Iran với Mỹ - Israel.

Theo ông Rezaei, cố vấn quân sự cấp cao của Lãnh tụ Tối cao Iran Mojtaba Khamenei, cuộc xung đột sẽ không kết thúc cho đến khi tất cả lệnh trừng phạt nhằm vào Iran được dỡ bỏ, tất cả thiệt hại của Iran được bồi thường và các đảm bảo quốc tế ràng buộc về mặt pháp lý được đưa ra để đảm bảo không có thêm bất kỳ hành động gây hấn nào nhằm vào Iran trong tương lai.

Ông Rezaei cho biết Iran không chấp nhận “lệnh ngừng bắn” tạm thời và sẵn sàng đáp trả các cuộc tấn công vào cơ sở hạ tầng của nước này.

“Nếu họ phạm sai lầm, chúng tôi sẽ làm tê liệt họ và nhấn chìm họ ở Vịnh Ba Tư”, ông Rezaei cảnh báo, đáp trả tuyên bố gần đây của Tổng thống Mỹ Donald Trump về các cuộc tấn công vào cơ sở hạ tầng của Iran.

Thông tin được đưa ra sau khi Tổng thống Mỹ Donald Trump hôm 23/3 thông báo hoãn kế hoạch tấn công hạ tầng năng lượng Iran trong 5 ngày.

Tổng thống Trump cho biết chính quyền của ông đang tiến hành các cuộc đàm phán hiệu quả với Iran về một “giải pháp hoàn toàn và triệt để” cho cuộc xung đột hiện nay.

Ông Trump nói rằng các bên đang tiến gần tới một thỏa thuận, theo đó Iran sẽ đồng ý không theo đuổi vũ khí hạt nhân và từ bỏ hoạt động làm giàu uranium, đồng thời các điều khoản này sẽ khiến Israel “rất hài lòng”.

Tuy nhiên, Iran phủ nhận việc các cuộc đàm phán đang diễn ra, bất chấp các báo cáo về những cuộc thương lượng gián tiếp giữa các bên.

TIN LIÊN QUAN
Iran phủ nhận tuyên bố của Tổng thống Trump về đàm phán đồng thuận 15 điểm
Reuters dẫn nguồn tin từ 3 quan chức cấp cao của Israel cho biết Tổng thống Trump dường như quyết tâm đạt được thỏa thuận với Iran nhằm chấm dứt xung đột ở Trung Đông.

Tuy nhiên, các quan chức Israel thừa nhận khó có khả năng Iran sẽ đồng ý với các yêu cầu của Mỹ trong bất kỳ vòng đàm phán mới nào. Vòng đàm phán trước đó đã đổ vỡ vào ngày 28/2 khi Mỹ và Israel phát động cuộc chiến chống lại Iran.

Những yêu cầu của Mỹ có thể bao gồm việc hạn chế các chương trình hạt nhân và tên lửa đạn đạo của Iran.

Sau tuyên bố tạm dừng tấn công hạ tầng năng lượng Iran, Tổng thống Trump nêu rõ điều kiện rằng Tehran không được sở hữu vũ khí hạt nhân.

“Chúng tôi không muốn thấy bom hạt nhân, vũ khí hạt nhân. Chúng tôi muốn thấy hòa bình ở Trung Đông”, ông nói, đồng thời đưa ra yêu cầu rằng Mỹ phải nắm giữ uranium được làm giàu cao của Iran.

Chủ nhân Nhà Trắng cũng nói rõ rằng việc chấm dứt chương trình hạt nhân của Iran là điều tối quan trọng đối với bất kỳ thỏa thuận nào và khẳng định Iran đã đồng ý với điều đó.

TIN LIÊN QUAN
Iran bác tin đàm phán, nói chuẩn bị "bất ngờ" dành cho Mỹ - Israel
Iran bác tin đàm phán, nói chuẩn bị "bất ngờ" dành cho Mỹ - Israel
Tổng thống Iran Masoud Pezeshkian từng tuyên bố bất kỳ thỏa thuận nào nhằm chấm dứt cuộc chiến với Mỹ và Israel phải bao gồm điều khoản bồi thường thiệt hại và các bảo đảm an ninh cho Iran và công nhận các quyền hợp pháp của Iran.

Đại sứ Iran tại Nga Kazem Jalali cũng nêu rõ 3 điều kiện để Iran quay trở lại đàm phán và chấm dứt cuộc xung đột với Mỹ - Israel.

Ông nói rằng trước hết "xung đột phải chấm dứt và không bao giờ xảy ra lần thứ 3". Thứ hai, "tất cả các lệnh trừng phạt nhằm vào Iran cần phải được dỡ bỏ”. Thứ ba, “Iran phải được bồi thường cho tất cả những thiệt hại mà Iran đã phải gánh chịu”.

Chiến dịch của Mỹ và Israel lần này đánh dấu lần tấn công thứ hai nhằm vào Iran trong vòng chưa đầy 12 tháng. Cuộc xung đột đầu tiên kéo dài 12 ngày vào tháng 6 năm ngoái.
		`,
		`Tổng thống Mỹ khẳng định các cuộc đàm phán đang diễn ra với Iran, trong khi giới chức Tehran tuyên bố chiến sự chưa kết thúc.
Mỹ tuyên bố đang đàm phán chấm dứt chiến tranh, nêu rõ điều kiện với Iran - 1
Tổng thống Mỹ Donald Trump phát biểu trước truyền thông về tình hình Iran trước khi rời West Palm Beach ngày 23/3 (Ảnh: Reuters).

Mỹ muốn đạt thỏa thuận với Iran

Trao đổi với các phóng viên tại sân bay ở West Palm Beach, Tổng thống Donald Trump xác nhận Mỹ và Iran đang đàm phán về việc chấm dứt chiến tranh.

Ông Trump nói rằng hai bên đã đạt được “những điểm đồng thuận quan trọng” sau các cuộc đàm phán kéo dài đến tối 22/3 (giờ Mỹ) với hai đặc phái viên hàng đầu của Mỹ gồm ông Steve Witkoff và ông Jared Kushner, con rể của Tổng thống Trump.

“Tôi cho rằng, các cuộc đàm phán diễn ra hoàn hảo”, ông nói thêm, đồng thời cho biết Iran đã khởi xướng các cuộc đàm phán.

Theo nhà lãnh đạo Mỹ, nếu các cuộc đàm phán thành công, xung đột có thể được giải quyết.

“Tôi cho rằng nếu họ tiếp tục như vậy, xung đột sẽ chấm dứt và tôi nghĩ sẽ chấm dứt một cách rất triệt để”, ông Trump nhấn mạnh.

Khi được hỏi Mỹ đang đàm phán với ai ở Iran, Tổng thống Trump cho biết đó là một “nhân vật cấp cao” trong chính quyền Iran, một nhà lãnh đạo “được kính trọng”, nhưng không phải là tân Lãnh tụ Tối cao Mojtaba Khamenei.

Khi được hỏi dồn về việc liệu Mỹ có đang đàm phán với ông Mojtaba hay không, Tổng thống Trump nói: “Không, không phải là Lãnh tụ Tối cao”.

“Chúng tôi không biết liệu ông ấy còn sống hay không”, ông Trump nói về Lãnh tụ Tối cao mới của Iran.

Trước đó, giới chức Mỹ cho rằng ông Mojtaba Khamenei đã bị thương nặng trong cuộc tập kích của Mỹ và Israel. Tuy nhiên, phía Iran phủ nhận thông tin này.

"Iran không được sở hữu vũ khí hạt nhân"

“Họ (Iran) rất muốn đạt được một thỏa thuận. Chúng tôi cũng muốn đạt được một thỏa thuận”, ông Trump nói, đồng thời cho biết sẽ có thêm các cuộc điện đàm vào ngày 23/3, tiếp theo là một cuộc gặp trực tiếp trong khoảng thời gian “rất sớm”.

“Chúng ta sẽ có khoảng thời gian 5 ngày, để xem mọi chuyện diễn ra thế nào. Nếu mọi việc suôn sẻ, chúng ta sẽ đạt được thỏa thuận. Nếu không, chúng ta sẽ tiếp tục ném bom”, ông Trump cho biết.

Sau tuyên bố tạm dừng tấn công hạ tầng năng lượng Iran, Tổng thống Trump nêu rõ điều kiện rằng Tehran không được sở hữu vũ khí hạt nhân.

“Chúng tôi không muốn thấy bom hạt nhân, vũ khí hạt nhân. Chúng tôi muốn thấy hòa bình ở Trung Đông”, ông nói, đồng thời đưa ra yêu cầu rằng Mỹ phải nắm giữ uranium được làm giàu cao của Iran.

Chủ nhân Nhà Trắng cũng nói rõ rằng việc chấm dứt chương trình hạt nhân của Iran là điều tối quan trọng đối với bất kỳ thỏa thuận nào và khẳng định Iran đã đồng ý với điều đó.

"Chúng tôi rất sẵn lòng đạt được một thỏa thuận. Đó phải là một thỏa thuận tốt và không còn chiến tranh, không còn vũ khí hạt nhân. Họ sẽ không còn vũ khí hạt nhân nữa. Họ đã đồng ý với điều đó”, ông Trump nêu rõ.

TIN LIÊN QUAN
Mỹ tạm dừng tấn công hạ tầng năng lượng của Iran trong 5 ngày
Tổng thống Mỹ cho biết, có thể nhìn nhận rằng Iran đã và đang trải qua một cuộc thay đổi chế độ vì các cuộc tấn công của Mỹ và Israel đã nhắm mục tiêu và loại bỏ nhiều nhà lãnh đạo cấp cao của Iran.

Trong cuộc phỏng vấn qua điện thoại với CNBC, Tổng thống Trump cũng khẳng định các cuộc đàm phán đang diễn ra giữa Mỹ và Iran.

“Chúng tôi rất quyết tâm đạt được thỏa thuận với Iran”, Tổng thống Mỹ nhấn mạnh, đồng thời cho biết các cuộc thảo luận với chính quyền Iran diễn ra “rất căng thẳng”.

Theo người dẫn chương trình Maria Bartiromo của Fox Business, Tổng thống Trump nói rằng “Iran rất muốn đạt được một thỏa thuận” với Mỹ.

Trước đó, Tổng thống Trump thông báo Mỹ đang đàm phán với Iran để giải quyết xung đột.

"Tôi rất vui mừng thông báo rằng, trong 2 ngày qua, Mỹ và Iran đã có những cuộc đàm phán rất tốt đẹp và hiệu quả về việc giải quyết toàn diện, triệt để các hành động thù địch ở Trung Đông", Tổng thống Trump viết trên mạng xã hội.

"Dựa trên tinh thần và không khí của những cuộc trao đổi đó, tôi đã chỉ thị Bộ Chiến tranh hoãn tất cả các cuộc tấn công quân sự nhằm vào các nhà máy điện và cơ sở hạ tầng năng lượng của Iran trong thời hạn 5 ngày, tùy thuộc vào sự thành công của các cuộc họp và thảo luận đang diễn ra", ông Trump cho biết thêm.

Nhà lãnh đạo Mỹ cũng cho biết các cuộc đàm phán "sâu rộng, chi tiết và mang tính xây dựng" với Iran sẽ tiếp diễn trong tuần này.

Xung đột vẫn tiếp diễn

Phản hồi tuyên bố của Tổng thống Trump, Bộ Ngoại giao Iran cho biết “không có cuộc đối thoại nào giữa Tehran và Washington”.

TIN LIÊN QUAN
Phản ứng của Iran khi Mỹ tuyên bố tạm dừng tấn công hạ tầng năng lượng
Phản ứng của Iran khi Mỹ tuyên bố tạm dừng tấn công hạ tầng năng lượng
Bộ Ngoại giao Iran nhấn mạnh các tuyên bố của Tổng thống Trump “là một phần trong nỗ lực giảm giá năng lượng và câu giờ để thực hiện các kế hoạch quân sự của ông”.

"Iran vẫn giữ vững lập trường bác bỏ mọi hình thức đàm phán trước khi đạt được các mục tiêu trong cuộc chiến", Bộ Ngoại giao Iran nêu rõ.

Theo Đại sứ quán Iran tại Afghanistan, Tổng thống Trump đã “nhượng bộ” sau “cảnh báo cứng rắn” của Iran rằng nước này sẽ trả đũa các cuộc tấn công vào cơ sở hạ tầng năng lượng Iran bằng cách tấn công các nhà máy điện trên khắp khu vực.

Hãng thông tấn nhà nước Iran Fars dẫn lời một quan chức cấp cao của Iran tiết lộ Tổng thống Trump đã nhượng bộ sau khi nghe tin Iran sẽ nhắm mục tiêu vào các nhà máy điện ở khu vực Tây Á.

Người phát ngôn của Ủy ban An ninh Quốc gia và Chính sách Đối ngoại Iran Ebrahim Rezaei tuyên bố “cuộc chiến vẫn tiếp diễn”.

Người phát ngôn cho biết việc Tổng thống Trump tuyên bố hoãn các cuộc tấn công vào các nhà máy điện của Iran và khẳng định về các cuộc đàm phán giữa Washington và Tehran là “một thất bại nữa” của Mỹ.

“Ông Trump và Mỹ lại một lần nữa bị đánh bại”, ông Rezaei nói thêm.

Lực lượng Phòng vệ Israel (IDF) thông báo một đợt tấn công mới nhằm vào cơ sở hạ tầng ở Tehran, ngay sau khi Tổng thống Trump nói rằng các cuộc tấn công của Mỹ vào các nhà máy điện của Iran sẽ bị hoãn lại 5 ngày.

`,
	}

	for _, text := range corpus {
		// dp, err := eng.Add(ctx, text, engine.WithWaitAdd(true), engine.WithConsistencyThreshold(0.5))
		// must(err, "Add")
		fmt.Printf("Added to memory: %s...\n", text[:40])
		// if _, err := eng.Cognify(ctx, dp, engine.WithWaitCognify(true)); err != nil {
		// 	log.Printf("Warning: cognify failed for %s: %v", dp.ID, err)
		// }
		// if err := eng.Memify(ctx, dp, engine.WithWaitMemify(true)); err != nil {
		// 	log.Printf("Warning: memify failed for %s: %v", dp.ID, err)
		// }
	}
	// ─── 6. Graph Traversal ───────────────────────────────────────────────────
	fmt.Println("\n=== GRAPH TRAVERSAL RESULTS ===")

	question := "Ông Trump định làm gì"

	fmt.Println("\n-- Thinking about: '" + question + "'")
	thinkResult, err := eng.Think(ctx, &schema.ThinkQuery{
		Text:      question,
		SessionID: sessionID,
		Limit:     3,
		HopDepth:  2,
	})
	must(err, "think")

	fmt.Printf("\n🤔 AI Reasoning:\n%s\n", thinkResult.Reasoning)
	fmt.Printf("\n💡 AI Answer:\n%s\n", thinkResult.Answer)

	fmt.Println("\n✅ Knowledge Graph Builder Example Complete.")
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("FATAL %s: %v", label, err)
	}
}
