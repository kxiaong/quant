import psycopg2

from datetime import datetime, timedelta,timezone

def connect_db():
    dbconf = dict(dbname='coinbene', user='coinbene', host='localhost',port='5432', password='xiaoxiao')
    conn = psycopg2.connect(**dbconf)
    return conn

def get_trade_records(conn):
    cursor = conn.cursor()
    since = datetime.utcnow() - timedelta(hours=1)
    since = since.strftime("%Y-%m-%d %H:%M:%S")
    sql = f"SELECT price, direction, sum(amount) FROM coinbene_trade_list where ts > '{since}' group by price, direction;"
    print(sql)
    cursor.execute(sql)
    result = cursor.fetchall()
    print(result)


if __name__ == "__main__":
    conn = connect_db()
    get_trade_records(conn)
