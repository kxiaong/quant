import psycopg2
import redis
import json
import time

from datetime import datetime, timedelta,timezone

def connect_db():
    dbconf = dict(dbname='coinbene', user='coinbene', host='localhost',port='5432', password='xiaoxiao')
    conn = psycopg2.connect(**dbconf)
    return conn

def connect_redis():
    rdconf = dict(host="localhost", port=6379)
    return redis.Redis(**rdconf)

def get_trade_records(conn):
    cursor = conn.cursor()
    since = datetime.utcnow() - timedelta(hours=1)
    since = since.strftime("%Y-%m-%d %H:%M:%S")
    sql = f"SELECT price, direction, sum(amount) FROM coinbene_trade_list where ts > '{since}' group by price, direction;"
    cursor.execute(sql)
    result = cursor.fetchall()
    return result


def get_latest_ticker(rdconn):
    s = rdconn.get("coinbene:ticker:BTC-SWAP")
    return json.loads(s)


def calculate_current_income(trade_records, ticker):
    buyser_income = 0.0
    buyser_count = 0
    seller_income = 0.0
    seller_count = 0
    latest_price = float(ticker['lastPrice'])
    for e in trade_records:
        btc_amount = e[2] * 0.001
        if e[1] == "buy":
            # 买方收益
            diff = latest_price - float(e[0])
            diff_income = diff * btc_amount
            buyser_income += diff_income
            buyser_count +=1
        else:
            #卖方收益
            diff = latest_price - float(e[0])
            diff_income = diff * btc_amount
            seller_income += diff_income
            seller_count += 1
    print(f"buyser income: {buyser_income}, buyser avg income: {buyser_income/buyser_count}")
    print(f"seller income: {seller_income}, seller avg income: {seller_income/seller_count}")
    return seller_income, seller_count, buyser_income, buyser_count

def calculate_30s_trade(conn):
    cursor = conn.cursor()
    since = datetime.utcnow() - timedelta(seconds=30)
    since = since.strftime("%Y-%m-%d %H:%M:%S")
    sql = f"SELECT direction, sum(amount) FROM coinbene_trade_list where ts >='{since}' group by direction;"
    print(sql)
    cursor.execute(sql)
    result = cursor.fetchall()
    print(result)


if __name__ == "__main__":
    while True:
        conn = connect_db()
        rdconn = connect_redis()

        calculate_30s_trade(conn)

        ticker = get_latest_ticker(rdconn)
        records = get_trade_records(conn)
        seller_income, seller_count, buyser_income, buyser_count = calculate_current_income(records, ticker)
        seller_avg_income = seller_income / seller_count
        buyser_avg_income = buyser_income / buyser_count



        diff = abs(seller_avg_income - buyser_avg_income)
        if diff > 50:
            print("signal to trade")
        else:
            print("waiting for signal")
        print("*"*12)
        time.sleep(3)
