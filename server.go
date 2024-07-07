package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type CotacaoResponse struct {
	USDBRL Cotacao `json:"USDBRL"`
}

type Cotacao struct {
	Bid string `json:"bid"`
}

func getCotacao(ctx context.Context) (Cotacao, error) {
	var cotacaoResp CotacaoResponse
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://economia.awesomeapi.com.br/json/last/USD-BRL", nil)
	if err != nil {
		return cotacaoResp.USDBRL, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return cotacaoResp.USDBRL, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&cotacaoResp); err != nil {
		return cotacaoResp.USDBRL, err
	}

	return cotacaoResp.USDBRL, nil
}

func saveCotacao(ctx context.Context, db *sql.DB, cotacao Cotacao) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		_, err := db.ExecContext(ctx, "INSERT INTO cotacoes (bid) VALUES (?)", cotacao.Bid)
		return err
	}
}

func cotacaoHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "./cotacoes.db")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	ctxAPI, cancelAPI := context.WithTimeout(r.Context(), 200*time.Millisecond)
	defer cancelAPI()

	cotacao, err := getCotacao(ctxAPI)
	if err != nil {
		log.Println("Erro ao obter cotação:", err)
		w.Header().Set("Content-Type", "application/text")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctxDB, cancelDB := context.WithTimeout(r.Context(), 10*time.Millisecond)
	defer cancelDB()

	err = saveCotacao(ctxDB, db, cotacao)
	if err != nil {
		log.Println("Erro ao salvar cotação:", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cotacao)
}

func main() {
	db, err := sql.Open("sqlite3", "./cotacoes.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS cotacoes (id INTEGER PRIMARY KEY, bid TEXT)")
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/cotacao", cotacaoHandler)
	log.Println("Servidor iniciado na porta 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
