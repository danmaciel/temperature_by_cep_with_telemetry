# API em Golang que retorna a temperatura.

Sistema que recebe um CEP e retorna a temperatura em Celsius, Fahrenheit e Kelvin, por meio de um CEP de entrada e utiliza telemetria via OpenTelemetry e Zipkin

O retorno dos dados e no formato JSON e na seguinte estrutura:
```
{ "city: "São Paulo", "temp_C": 28.5, "temp_F": 28.5, "temp_K": 28.5 }
```

Onde:

```
city   = a cidade do cep pesquisado 
temp_c = temperatura em Celsius
temp_f = temperatura em Farenheit
temp_k = temperatura em Kelvin
```

Exemplo de saída:
```
    {
    "city": "Londrina",
    "temp_c": 25.2,
    "temp_f": 77.36,
    "temp_k": 298.2
    }
```

## O que é necessário para rodar?

```
Ter instalado o Go na sua máquina.
Ter instalado o Docker na sua máquina.
Cadastro em https://www.weatherapi.com para ter a API key para utilização na aplicação.
```


## Como rodar?

Na raiz da aplicação, edite o arquivo docker-compose.yml adicionando em services -> service_b -> environment -> WEATHER_API_KEY e adicione sua key fornecida previamente em weatherapi.com, depois, com o serviço do docker ativo, execute no terminal.

```
docker-compose up
```

Com os serviços iniciados, faça uma requisição do tipo POST para http://localhost:8080/cep passando no corpo o parâmetro CEP no formato:

```
    {
       "cep": "86027410"
    }
```

Para acessar o ZipKin para a exibição do tracer, abra no browser o endereço 

```
http://localhost:9411/zipkin/
```

E execute uma requisição.
No diretório API na raiz da aplicação, possui um arquivo http que pode ser usado no teste de requisição.

Depois da requisição realizada, clique algumas vezes no botão "Run Query" até que seja exibido o trace com as informações.


